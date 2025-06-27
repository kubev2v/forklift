// Copyright 2022 NetApp, Inc. All Rights Reserved.

package utils

import (
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"

	. "github.com/netapp/trident/logging"
	"github.com/netapp/trident/utils/exec"
)

var (
	iqnRegex              = regexp.MustCompile(`^\s*InitiatorName\s*=\s*(?P<iqn>\S+)(|\s+.*)$`)
	xtermControlRegex     = regexp.MustCompile(`\x1B\[[0-9;]*[a-zA-Z]`)
	portalPortPattern     = regexp.MustCompile(`.+:\d+$`)
	pidRunningOrIdleRegex = regexp.MustCompile(`pid \d+ (running|idle)`)
	pidRegex              = regexp.MustCompile(`^\d+$`)
	deviceRegex           = regexp.MustCompile(`/dev/(?P<device>[\w-]+)`)

	chrootPathPrefix string

	// FIXME: Instead of a package-level variable, pass command into other utils once their interfaces are defined.
	command = exec.NewCommand()
)

const devMapperRoot = "/dev/mapper/"

func init() {
	if os.Getenv("DOCKER_PLUGIN_MODE") != "" {
		SetChrootPathPrefix("/host")
	} else {
		SetChrootPathPrefix("")
	}
}

func SetChrootPathPrefix(prefix string) {
	Logc(context.Background()).Debugf("SetChrootPathPrefix = '%s'", prefix)
	chrootPathPrefix = prefix
}

// GetIPAddresses returns the sorted list of Global Unicast IP addresses available to Trident
func GetIPAddresses(ctx context.Context) ([]string, error) {
	Logc(ctx).Debug(">>>> osutils.GetIPAddresses")
	defer Logc(ctx).Debug("<<<< osutils.GetIPAddresses")

	ipAddrs := make([]string, 0)
	addrsMap := make(map[string]struct{})

	// Get the set of potentially viable IP addresses for this host in an OS-appropriate way.
	addrs, err := getIPAddresses(ctx)
	if err != nil {
		err = fmt.Errorf("could not gather system IP addresses; %v", err)
		Logc(ctx).Error(err)
		return nil, err
	}

	// Strip netmask and use a map to ensure addresses are deduplicated.
	for _, addr := range addrs {

		// net.Addr are of form 1.2.3.4/32, but IP needs 1.2.3.4, so we must strip the netmask (also works for IPv6)
		parsedAddr := net.ParseIP(strings.Split(addr.String(), "/")[0])

		Logc(ctx).WithField("IPAddress", parsedAddr.String()).Debug("Discovered potentially viable IP address.")

		addrsMap[parsedAddr.String()] = struct{}{}
	}

	for addr := range addrsMap {
		ipAddrs = append(ipAddrs, addr)
	}
	sort.Strings(ipAddrs)
	return ipAddrs, nil
}

func PathExists(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return true, nil
	}
	return false, nil
}

// EnsureFileExists makes sure that file of given name exists
func EnsureFileExists(ctx context.Context, path string) error {
	fields := LogFields{"path": path}
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			Logc(ctx).WithFields(fields).Error("Path exists but is a directory")
			return fmt.Errorf("path exists but is a directory: %s", path)
		}
		return nil
	} else if !os.IsNotExist(err) {
		Logc(ctx).WithFields(fields).Errorf("Can't determine if file exists; %s", err)
		return fmt.Errorf("can't determine if file %s exists; %s", path, err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC, 0o600)
	if nil != err {
		Logc(ctx).WithFields(fields).Errorf("OpenFile failed; %s", err)
		return fmt.Errorf("failed to create file %s; %s", path, err)
	}
	file.Close()

	return nil
}

// DeleteResourceAtPath makes sure that given named file or (empty) directory is removed
func DeleteResourceAtPath(ctx context.Context, resource string) error {
	fields := LogFields{"resource": resource}

	// Check if resource exists
	if _, err := os.Stat(resource); err != nil {
		if os.IsNotExist(err) {
			Logc(ctx).WithFields(fields).Debugf("Resource not found.")
			return nil
		} else {
			Logc(ctx).WithFields(fields).Debugf("Can't determine if resource exists; %s", err)
			return fmt.Errorf("can't determine if resource %s exists; %s", resource, err)
		}
	}

	// Remove resource
	if err := os.Remove(resource); err != nil {
		Logc(ctx).WithFields(fields).Debugf("Failed to remove resource, %s", err)
		return fmt.Errorf("failed to remove resource %s; %s", resource, err)
	}

	return nil
}

// WaitForResourceDeletionAtPath accepts a resource name and waits until it is deleted and returns error if it times out
func WaitForResourceDeletionAtPath(ctx context.Context, resource string, maxDuration time.Duration) error {
	fields := LogFields{"resource": resource}
	Logc(ctx).WithFields(fields).Debug(">>>> osutils.WaitForResourceDeletionAtPath")
	defer Logc(ctx).WithFields(fields).Debug("<<<< osutils.WaitForResourceDeletionAtPath")

	checkResourceDeletion := func() error {
		return DeleteResourceAtPath(ctx, resource)
	}

	deleteNotify := func(err error, duration time.Duration) {
		Logc(ctx).WithField("increment", duration).Debug("Resource not deleted yet, waiting.")
	}

	deleteBackoff := backoff.NewExponentialBackOff()
	deleteBackoff.InitialInterval = 1 * time.Second
	deleteBackoff.Multiplier = 1.414 // approx sqrt(2)
	deleteBackoff.RandomizationFactor = 0.1
	deleteBackoff.MaxElapsedTime = maxDuration

	// Run the check using an exponential backoff
	if err := backoff.RetryNotify(checkResourceDeletion, deleteBackoff, deleteNotify); err != nil {
		return fmt.Errorf("could not delete resource after %3.2f seconds", maxDuration.Seconds())
	} else {
		Logc(ctx).WithField("resource", resource).Debug("Resource deleted.")
		return nil
	}
}

// EnsureDirExists makes sure that given directory structure exists
func EnsureDirExists(ctx context.Context, path string) error {
	fields := LogFields{
		"path": path,
	}
	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() {
			Logc(ctx).WithFields(fields).Error("Path exists but is not a directory")
			return fmt.Errorf("path exists but is not a directory: %s", path)
		}
		return nil
	} else if !os.IsNotExist(err) {
		Logc(ctx).WithFields(fields).Errorf("Can't determine if directory exists; %s", err)
		return fmt.Errorf("can't determine if directory %s exists; %s", path, err)
	}

	err := os.MkdirAll(path, 0o755)
	if err != nil {
		Logc(ctx).WithFields(fields).Errorf("Mkdir failed; %s", err)
		return fmt.Errorf("failed to mkdir %s; %s", path, err)
	}

	return nil
}
