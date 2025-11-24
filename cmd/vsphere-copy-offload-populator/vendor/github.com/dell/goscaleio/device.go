// Copyright © 2019 - 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"time"

	types "github.com/dell/goscaleio/types/v1"
)

// Device defines struct for Device
type Device struct {
	Device *types.Device
	client *Client
}

// NewDevice returns a new Device
func NewDevice(client *Client) *Device {
	return &Device{
		Device: &types.Device{},
		client: client,
	}
}

// NewDeviceEx returns a new Device
func NewDeviceEx(client *Client, device *types.Device) *Device {
	return &Device{
		Device: device,
		client: client,
	}
}

// AttachDevice attaches a device
func (sp *StoragePool) AttachDevice(deviceParam *types.DeviceParam) (string, error) {
	defer TimeSpent("AttachDevice", time.Now())
	deviceParam.StoragePoolID = sp.StoragePool.ID
	dev := types.DeviceResp{}
	err := sp.client.getJSONWithRetry(
		http.MethodPost, "/api/types/Device/instances",
		deviceParam, &dev)
	if err != nil {
		return "", err
	}

	return dev.ID, nil
}

// GetDevice returns a device based on Storage Pool ID
func (sp *StoragePool) GetDevice() ([]types.Device, error) {
	defer TimeSpent("GetDevice", time.Now())

	path := fmt.Sprintf(
		"/api/instances/StoragePool::%v/relationships/Device",
		sp.StoragePool.ID)

	var devices []types.Device
	err := sp.client.getJSONWithRetry(
		http.MethodGet, path, nil, &devices)
	if err != nil {
		return nil, err
	}

	return devices, nil
}

// FindDevice returns a Device
func (sp *StoragePool) FindDevice(
	field, value string,
) (*types.Device, error) {
	defer TimeSpent("FindDevice", time.Now())

	devices, err := sp.GetDevice()
	if err != nil {
		return nil, err
	}

	for _, device := range devices {
		valueOf := reflect.ValueOf(device)
		switch {
		case reflect.Indirect(valueOf).FieldByName(field).String() == value:
			return &device, nil
		}
	}

	return nil, errors.New("couldn't find device")
}

// GetDevice returns a devices based on SDS ID
func (sds *Sds) GetDevice() ([]types.Device, error) {
	defer TimeSpent("GetSDSDevice", time.Now())

	path := fmt.Sprintf(
		"/api/instances/Sds::%v/relationships/Device",
		sds.Sds.ID)

	var devices []types.Device
	err := sds.client.getJSONWithRetry(http.MethodGet, path, nil, &devices)
	if err != nil {
		return nil, err
	}

	return devices, nil
}

// FindDevice returns a Device
func (sds *Sds) FindDevice(
	field, value string,
) (*types.Device, error) {
	defer TimeSpent("FindDevice", time.Now())

	devices, err := sds.GetDevice()
	if err != nil {
		return nil, err
	}

	for _, device := range devices {
		valueOf := reflect.ValueOf(device)
		switch {
		case reflect.Indirect(valueOf).FieldByName(field).String() == value:
			return &device, nil
		}
	}

	return nil, errors.New("couldn't find device")
}

// GetAllDevice returns all device in the system
func (s *System) GetAllDevice() ([]types.Device, error) {
	defer TimeSpent("GetAllDevice", time.Now())

	path := "/api/types/Device/instances"

	var deviceResult []types.Device
	err := s.client.getJSONWithRetry(
		http.MethodGet, path, nil, &deviceResult)
	if err != nil {
		return nil, err
	}

	return deviceResult, nil
}

// GetDeviceByField returns a Device list filter by the field
func (s *System) GetDeviceByField(
	field, value string,
) ([]types.Device, error) {
	defer TimeSpent("GetDeviceByField", time.Now())

	devices, err := s.GetAllDevice()
	if err != nil {
		return nil, err
	}

	var filterdevices []types.Device
	for _, device := range devices {
		valueOf := reflect.ValueOf(device)
		if reflect.Indirect(valueOf).FieldByName(field).String() == value {
			filterdevices = append(filterdevices, device)
		}
	}
	if len(filterdevices) > 0 {
		return filterdevices, nil
	}

	return nil, errors.New("couldn't find device")
}

// GetDevice returns a device using Device ID
func (s *System) GetDevice(id string) (*types.Device, error) {
	defer TimeSpent("GetDevice", time.Now())

	path := fmt.Sprintf(
		"/api/instances/Device::%v",
		id)

	var deviceResult types.Device
	err := s.client.getJSONWithRetry(
		http.MethodGet, path, nil, &deviceResult)
	if err != nil {
		return nil, err
	}

	return &deviceResult, nil
}

// SetDeviceName modifies device name
func (sp *StoragePool) SetDeviceName(id, name string) error {
	defer TimeSpent("SetDeviceName", time.Now())

	deviceParam := &types.SetDeviceName{
		Name: name,
	}
	path := fmt.Sprintf("/api/instances/Device::%v/action/setDeviceName", id)

	err := sp.client.getJSONWithRetry(
		http.MethodPost, path, deviceParam, nil)
	if err != nil {
		return err
	}
	return nil
}

// SetDeviceMediaType modifies device media type
func (sp *StoragePool) SetDeviceMediaType(id, mediaType string) error {
	defer TimeSpent("SetDeviceMediaType", time.Now())

	deviceParam := &types.SetDeviceMediaType{
		MediaType: mediaType,
	}
	path := fmt.Sprintf("/api/instances/Device::%v/action/setMediaType", id)

	err := sp.client.getJSONWithRetry(
		http.MethodPost, path, deviceParam, nil)
	if err != nil {
		return err
	}
	return nil
}

// SetDeviceExternalAccelerationType modifies device external acceleration type
func (sp *StoragePool) SetDeviceExternalAccelerationType(id, externalAccelerationType string) error {
	defer TimeSpent("SetDeviceExternalAccelerationType", time.Now())

	deviceParam := &types.SetDeviceExternalAccelerationType{
		ExternalAccelerationType: externalAccelerationType,
	}
	path := fmt.Sprintf("/api/instances/Device::%v/action/setExternalAccelerationType", id)

	err := sp.client.getJSONWithRetry(
		http.MethodPost, path, deviceParam, nil)
	if err != nil {
		return err
	}
	return nil
}

// SetDeviceCapacityLimit modifies device capacity limit
func (sp *StoragePool) SetDeviceCapacityLimit(id, capacityLimitInGB string) error {
	defer TimeSpent("SetDeviceExternalAccelerationType", time.Now())

	deviceParam := &types.SetDeviceCapacityLimit{
		DeviceCapacityLimit: capacityLimitInGB,
	}
	path := fmt.Sprintf("/api/instances/Device::%v/action/setDeviceCapacityLimit", id)

	err := sp.client.getJSONWithRetry(
		http.MethodPost, path, deviceParam, nil)
	if err != nil {
		return err
	}
	return nil
}

// UpdateDeviceOriginalPathways modifies device path if changed during server restart
func (sp *StoragePool) UpdateDeviceOriginalPathways(id string) error {
	defer TimeSpent("UpdateDeviceOriginalPathways", time.Now())

	path := fmt.Sprintf("/api/instances/Device::%v/action/updateDeviceOriginalPathname", id)
	deviceParam := &types.EmptyPayload{}

	err := sp.client.getJSONWithRetry(
		http.MethodPost, path, deviceParam, nil)
	if err != nil {
		return err
	}
	return nil
}

// RemoveDevice removes device from storage pool
func (sp *StoragePool) RemoveDevice(id string) error {
	defer TimeSpent("RemoveDevice", time.Now())

	path := fmt.Sprintf("/api/instances/Device::%v/action/removeDevice", id)

	err := sp.client.getJSONWithRetry(
		http.MethodPost, path, nil, nil)
	if err != nil {
		return err
	}
	return nil
}
