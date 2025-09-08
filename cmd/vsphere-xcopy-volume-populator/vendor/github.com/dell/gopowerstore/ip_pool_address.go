/*
 *
 * Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package gopowerstore

import (
	"context"
	"errors"
	"fmt"
)

const apiPoolAddressURL = "ip_pool_address"

// GetStorageISCSITargetAddresses returns a list of PowerStore iSCSI targets ip addresses
func (c *ClientIMPL) GetStorageISCSITargetAddresses(
	ctx context.Context,
) (resp []IPPoolAddress, err error) {
	var ipPoolAddress IPPoolAddress
	qp := c.APIClient().QueryParamsWithFields(&ipPoolAddress)
	qp.RawArg("purposes", fmt.Sprintf("cs.{%s}", IPPurposeTypeEnumStorageIscsiTarget))
	qp.Order("id")
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    apiPoolAddressURL,
			QueryParams: qp,
		},
		&resp)
	err = WrapErr(err)
	if err != nil {
		return resp, err
	}
	if len(resp) == 0 {
		return resp, errors.New("can't get iscsi target address")
	}
	return resp, nil
}

// GetStorageNVMETCPTargetAddresses returns a list of PowerStore NVME/TCP targets ip addresses
func (c *ClientIMPL) GetStorageNVMETCPTargetAddresses(
	ctx context.Context,
) (resp []IPPoolAddress, err error) {
	var ipPoolAddress IPPoolAddress
	qp := c.APIClient().QueryParamsWithFields(&ipPoolAddress)
	qp.RawArg("purposes", fmt.Sprintf("cs.{%s}", IPPurposeTypeEnumStorageNVMETCPPort))
	qp.Order("id")
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    apiPoolAddressURL,
			QueryParams: qp,
		},
		&resp)
	err = WrapErr(err)
	if err != nil {
		return resp, err
	}
	if len(resp) == 0 {
		return resp, errors.New("can't get NVMeTCP target address")
	}
	return resp, nil
}
