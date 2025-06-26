//go:build !windows

// Copyright © 2021 - 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package goscaleio

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/google/uuid"
)

const (
	_IOCTLBase      = 'a'
	_IOCTLQueryGUID = 14
	_IOCTLQueryMDM  = 12
	_IOCTLRescan    = 10
	// IOCTLDevice is the default device to send queries to
	IOCTLDevice = "/dev/scini"
	mockGUID    = "9E56672F-2F4B-4A42-BFF4-88B6846FBFDA"
	mockSystem  = "14dbbf5617523654"
	drvCfg      = "/opt/emc/scaleio/sdc/bin/drv_cfg"
)

var (
	// SDCDevice is the device used to communicate with the SDC
	SDCDevice = IOCTLDevice
	// SCINIMockMode is used for testing upper layer code that attempts to call these methods
	SCINIMockMode = false
)

type ioctlGUID struct {
	rc         [8]byte
	uuid       [16]byte
	netIDMagic uint32
	netIDTime  uint32
}

// DrvCfgIsSDCInstalled will check to see if the SDC kernel module is loaded
func DrvCfgIsSDCInstalled() bool {
	if SCINIMockMode == true {
		return true
	}
	// Check to see if the SDC device is available
	info, err := os.Stat(SDCDevice)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// DrvCfgQueryGUID will return the GUID of the locally installed SDC
func DrvCfgQueryGUID() (string, error) {
	if SCINIMockMode == true {
		return mockGUID, nil
	}
	f, err := os.Open(SDCDevice)
	if err != nil {
		return "", err
	}

	defer func() {
		_ = f.Close()
	}()

	opCode := _IO(_IOCTLBase, _IOCTLQueryGUID)

	buf := [1]ioctlGUID{}
	// #nosec CWE-242, validated buffer is large enough to hold data
	err = ioctl(f.Fd(), opCode, uintptr(unsafe.Pointer(&buf[0])))
	if err != nil {
		return "", fmt.Errorf("QueryGUID error: %v", err)
	}

	rc, err := strconv.ParseInt(hex.EncodeToString(buf[0].rc[0:1]), 16, 64)
	if rc != 65 {
		return "", fmt.Errorf("Request to query GUID failed, RC=%d", rc)
	}

	g := hex.EncodeToString(buf[0].uuid[:len(buf[0].uuid)])
	u, err := uuid.Parse(g)
	discoveredGUID := strings.ToUpper(u.String())
	return discoveredGUID, nil
}

// DrvCfgQueryRescan preforms a rescan
func DrvCfgQueryRescan() (string, error) {
	f, err := os.Open(SDCDevice)
	if err != nil {
		return "", fmt.Errorf("Powerflex SDC is not installed")
	}

	defer func() {
		_ = f.Close()
	}()

	opCode := _IO(_IOCTLBase, _IOCTLRescan)

	var rc int64
	// #nosec CWE-242, validated buffer is large enough to hold data
	err = ioctl(f.Fd(), opCode, uintptr(unsafe.Pointer(&rc)))
	if err != nil {
		return "", fmt.Errorf("Rescan error: %v", err)
	}
	rcCode := strconv.FormatInt(rc, 10)

	return rcCode, err
}

// ConfiguredCluster contains configuration information for one connected system
type ConfiguredCluster struct {
	// SystemID is the MDM cluster system ID
	SystemID string
	// SdcID is the ID of the SDC as known to the MDM cluster
	SdcID string
}

// DrvCfgQuerySystems will return the configured MDM endpoints for the locally installed SDC
func DrvCfgQuerySystems() (*[]ConfiguredCluster, error) {
	clusters := make([]ConfiguredCluster, 0)

	if SCINIMockMode == true {
		systemID := mockSystem
		sdcID := mockGUID
		aCluster := ConfiguredCluster{
			SystemID: systemID,
			SdcID:    sdcID,
		}
		clusters = append(clusters, aCluster)
		return &clusters, nil
	}

	cmd := exec.Command(drvCfg, "--query_mdm")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("DrvCfgQuerySystems: Request to query MDM failed : %v", err)
	}

	// Parse the output to extract MDM information
	re := regexp.MustCompile(`MDM-ID ([a-f0-9]+) SDC ID ([a-f0-9]+)`)
	matches := re.FindAllStringSubmatch(string(output), -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no MDM information found in drv_cfg output")
	}

	// Fetch the systemID and sdcID for each system
	for _, match := range matches {
		systemID := match[1]
		sdcID := match[2]
		aCluster := ConfiguredCluster{
			SystemID: systemID,
			SdcID:    sdcID,
		}
		clusters = append(clusters, aCluster)
	}

	return &clusters, nil
}

func ioctl(fd, op, arg uintptr) error {
	_, _, ep := syscall.Syscall(syscall.SYS_IOCTL, fd, op, arg)
	if ep != 0 {
		return syscall.Errno(ep)
	}
	return nil
}

func _IO(t uintptr, nr uintptr) uintptr {
	return _IOC(0x0, t, nr, 0)
}

func _IOC(dir, t, nr, size uintptr) uintptr {
	return (dir << 30) | (t << 8) | nr | (size << 16)
}
