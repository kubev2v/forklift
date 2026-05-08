/*
Copyright 2019 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package waitforreboot hosts the Windows wait-for-reboot watcher used by the
// forklift-wait-for-reboot command. The directory uses a hyphen to match
// deployment naming; the package name is waitforreboot.
package waitforreboot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/rest"
	k8swebsocket "k8s.io/client-go/transport/websocket"
	cnv "kubevirt.io/api/core/v1"
)

const (
	defaultSignal        = "CONVERSION_DONE"
	defaultOverallSec    = 1800
	defaultRebootSec     = 300
	phasePollInterval    = 5 * time.Second
	vmiNotRunningSnippet = "VMI is not running"
)

// Config holds watcher settings loaded from environment variables.
type Config struct {
	Signal          string
	OverallTimeout  time.Duration
	RebootTimeout   time.Duration
	VMIName         string
	VMINamespace    string
	ConsoleProtocol string
}

// ParseConfig builds Config from process environment.
func ParseConfig() (*Config, error) {
	cfg := &Config{
		Signal:          os.Getenv("SIGNAL"),
		VMIName:         os.Getenv("VMI_NAME"),
		VMINamespace:    os.Getenv("VMI_NAMESPACE"),
		ConsoleProtocol: "plain.kubevirt.io",
	}
	if cfg.Signal == "" {
		cfg.Signal = defaultSignal
	}
	var err error
	cfg.OverallTimeout, err = durationFromEnvSec(os.Getenv, "TIMEOUT", defaultOverallSec)
	if err != nil {
		return nil, err
	}
	cfg.RebootTimeout, err = durationFromEnvSec(os.Getenv, "REBOOT_TIMEOUT", defaultRebootSec)
	if err != nil {
		return nil, err
	}
	if cfg.VMIName == "" {
		return nil, fmt.Errorf("VMI_NAME is required")
	}
	if cfg.VMINamespace == "" {
		return nil, fmt.Errorf("VMI_NAMESPACE is required")
	}
	return cfg, nil
}

func durationFromEnvSec(getenv func(string) string, key string, defaultSec int) (time.Duration, error) {
	v := getenv(key)
	if v == "" {
		return time.Duration(defaultSec) * time.Second, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid %s %q (expect non-negative integer seconds)", key, v)
	}
	return time.Duration(n) * time.Second, nil
}

const (
	// ExitSuccess means the watcher finished without a hard serial-timeout failure.
	ExitSuccess = 0
	// ExitSignalTimeout indicates the overall TIMEOUT elapsed before SIGNAL was observed.
	ExitSignalTimeout = 1
)

// Watch connects to the VMI serial console, waits for SIGNAL, then polls VMI
// phase for a reboot pattern (Running → non-Running → Running). Missing the
// reboot pattern before REBOOT_TIMEOUT still returns ExitSuccess (soft success).
func Watch(ctx context.Context, restCfg *rest.Config, lg *log.Logger, cfg *Config) int {
	if lg == nil {
		lg = log.Default()
	}
	overallCtx, cancelOverall := context.WithTimeout(ctx, cfg.OverallTimeout)
	defer cancelOverall()

	kvREST, err := kubevirtRESTClient(restCfg)
	if err != nil {
		lg.Printf("creating kubevirt REST client: %v", err)
		return ExitSignalTimeout
	}

	lg.Printf("waiting for signal %q on VMI %s/%s serial console (overall TIMEOUT=%s)",
		cfg.Signal, cfg.VMINamespace, cfg.VMIName, cfg.OverallTimeout)

	conn, err := connectSerialConsoleUntil(overallCtx, restCfg, cfg, lg)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			lg.Printf("overall timeout exceeded before observing serial signal %q", cfg.Signal)
			return ExitSignalTimeout
		}
		lg.Printf("giving up on serial console: %v", err)
		return ExitSignalTimeout
	}

	gotSignal := waitForSignalOnConn(overallCtx, conn, cfg.Signal, lg)
	_ = conn.Close()

	if gotSignal {
		cancelOverall()
	}

	if !gotSignal {
		if errors.Is(overallCtx.Err(), context.DeadlineExceeded) {
			lg.Printf("overall TIMEOUT elapsed without seeing signal %q", cfg.Signal)
		} else {
			lg.Printf("stopped waiting for signal %q before overall TIMEOUT (see errors above)", cfg.Signal)
		}
		return ExitSignalTimeout
	}

	lg.Printf("signal received; polling VMI.phase every %s for up to %s (want Running→non-Running→Running)",
		phasePollInterval, cfg.RebootTimeout)

	rebootCtx, cancelReboot := context.WithTimeout(ctx, cfg.RebootTimeout)
	defer cancelReboot()

	if rebootSeen := pollPhaseReboot(rebootCtx, kvREST, cfg.VMINamespace, cfg.VMIName, lg); rebootSeen {
		lg.Printf("detected reboot from VMI phase transitions; exiting 0")
	} else if errors.Is(rebootCtx.Err(), context.DeadlineExceeded) {
		lg.Printf("REBOOT_TIMEOUT reached without detecting phase reboot; exiting 0 (soft success)")
	} else {
		lg.Printf("stopped polling phase without detecting reboot pattern; exiting 0 (soft success)")
	}

	return ExitSuccess
}

func kubevirtRESTClient(cfg *rest.Config) (*rest.RESTClient, error) {
	gv := cnv.SchemeGroupVersion
	copyCfg := rest.CopyConfig(cfg)
	copyCfg.GroupVersion = &gv
	copyCfg.APIPath = "/apis"
	scheme := runtime.NewScheme()
	_ = cnv.AddToScheme(scheme)
	copyCfg.NegotiatedSerializer = serializer.NewCodecFactory(scheme).WithoutConversion()
	copyCfg.ContentType = runtime.ContentTypeJSON
	if copyCfg.UserAgent == "" {
		copyCfg.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	return rest.RESTClientFor(copyCfg)
}

func connectSerialConsoleUntil(ctx context.Context, restCfg *rest.Config, cfg *Config, lg *log.Logger) (*websocket.Conn, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rt, holder, err := k8swebsocket.RoundTripperFor(restCfg)
		if err != nil {
			return nil, err
		}
		apiHost := strings.TrimSuffix(restCfg.Host, "/")
		u := fmt.Sprintf("%s/apis/subresources.kubevirt.io/v1/namespaces/%s/virtualmachineinstances/%s/console",
			apiHost, cfg.VMINamespace, cfg.VMIName)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		conn, err := k8swebsocket.Negotiate(rt, holder, req, cfg.ConsoleProtocol)
		if err == nil {
			lg.Printf("connected to serial console subresource WebSocket")
			return conn, nil
		}
		if !isVMINotRunningDialErr(err) {
			return nil, err
		}
		lg.Printf("VMI is not running yet; retrying console connection in %s (%v)", phasePollInterval, err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(phasePollInterval):
		}
	}
}

func isVMINotRunningDialErr(err error) bool {
	var upgrade *httpstream.UpgradeFailureError
	if errors.As(err, &upgrade) && upgrade.Cause != nil {
		var st *apierrors.StatusError
		if errors.As(upgrade.Cause, &st) {
			return strings.Contains(st.ErrStatus.Message, vmiNotRunningSnippet)
		}
		return strings.Contains(upgrade.Cause.Error(), vmiNotRunningSnippet)
	}
	return strings.Contains(err.Error(), vmiNotRunningSnippet)
}

func waitForSignalOnConn(ctx context.Context, conn *websocket.Conn, signal string, lg *log.Logger) bool {
	keep := max(len(signal)*2, 4096)
	buf := ""

	for {
		deadline, ok := ctx.Deadline()
		if !ok {
			deadline = time.Now().Add(365 * 24 * time.Hour)
		}
		if err := conn.SetReadDeadline(deadline); err != nil {
			lg.Printf("set read deadline on serial websocket: %v", err)
			return false
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return false
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				if ctx.Err() != nil {
					return false
				}
			}
			lg.Printf("serial websocket read stopped: %v", err)
			return false
		}

		buf += string(msg)
		if len(buf) > keep {
			buf = buf[len(buf)-keep:]
		}
		if strings.Contains(buf, signal) {
			lg.Printf("serial signal matched %q", signal)
			return true
		}
		if ctx.Err() != nil {
			return false
		}
	}
}

func fetchVMIPhase(ctx context.Context, c *rest.RESTClient, namespace, name string) (cnv.VirtualMachineInstancePhase, error) {
	vmi := &cnv.VirtualMachineInstance{}
	err := c.Get().
		Namespace(namespace).
		Resource("virtualmachineinstances").
		Name(name).
		Do(ctx).
		Into(vmi)
	if err != nil {
		return "", err
	}
	return vmi.Status.Phase, nil
}

// pollPhaseReboot returns true if phase goes Running → distinct non-running → Running.
func pollPhaseReboot(ctx context.Context, c *rest.RESTClient, namespace, name string, lg *log.Logger) bool {
	t := time.NewTicker(phasePollInterval)
	defer t.Stop()

	sawRunning := false
	leftRunning := false

	for {
		phase, err := fetchVMIPhase(ctx, c, namespace, name)
		if err != nil {
			lg.Printf("GET virtualmachineinstance (phase poll): %v", err)
		} else {
			lg.Printf("VMI.phase=%s", phase)
			if phase == cnv.Running {
				if leftRunning {
					return true
				}
				sawRunning = true
			} else if sawRunning && phase != "" && phase != cnv.Running {
				if !leftRunning {
					lg.Printf("VMI left Running phase (now %s); waiting for return to Running", phase)
				}
				leftRunning = true
			}
		}

		select {
		case <-ctx.Done():
			return false
		case <-t.C:
		}
	}
}
