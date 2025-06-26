// Copyright © 2024 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	types "github.com/dell/goscaleio/types/v1"
	"github.com/google/uuid"
)

// DeployService used to deploy service
func (gc *GatewayClient) DeployService(deploymentName, deploymentDesc, serviceTemplateID, firmwareRepositoryID, nodes string) (*types.ServiceResponse, error) {
	defer TimeSpent("DeployService", time.Now())

	path := fmt.Sprintf("/Api/V1/FirmwareRepository/%v", firmwareRepositoryID)

	req, httpError := http.NewRequest(http.MethodGet, gc.host+path, nil)
	if httpError != nil {
		return nil, httpError
	}

	if gc.version == "4.0" {
		req.Header.Set("Authorization", "Bearer "+gc.token)

		err := setCookie(req.Header, gc.host)
		if err != nil {
			return nil, fmt.Errorf("Error While Handling Cookie: %s", err)
		}

	} else {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(gc.username+":"+gc.password)))
	}

	req.Header.Set("Content-Type", "application/json")

	client := gc.http
	httpResp, httpRespError := client.Do(req)
	if httpRespError != nil {
		return nil, httpRespError
	}

	responseString, _ := extractString(httpResp)

	if httpResp.StatusCode == 200 && responseString == "" {
		return nil, fmt.Errorf("Firmware Repository Not Found")
	}

	path = fmt.Sprintf("/Api/V1/ServiceTemplate/%v?forDeployment=true", serviceTemplateID)

	req, httpError = http.NewRequest(http.MethodGet, gc.host+path, nil)
	if httpError != nil {
		return nil, httpError
	}

	if gc.version == "4.0" {
		req.Header.Set("Authorization", "Bearer "+gc.token)

		err := setCookie(req.Header, gc.host)
		if err != nil {
			return nil, fmt.Errorf("Error While Handling Cookie: %s", err)
		}

	} else {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(gc.username+":"+gc.password)))
	}

	req.Header.Set("Content-Type", "application/json")

	client = gc.http
	httpResp, httpRespError = client.Do(req)
	if httpRespError != nil {
		return nil, httpRespError
	}

	responseString, _ = extractString(httpResp)

	if httpResp.StatusCode != http.StatusOK || responseString == "" {
		return nil, fmt.Errorf("Service Template Not Found")
	}

	var templateData map[string]interface{}
	parseError := json.Unmarshal([]byte(responseString), &templateData)
	if parseError != nil {
		return nil, fmt.Errorf("Error While Parsing Response Data For Template: %s", parseError)
	}

	configuredNode, _ := templateData["serverCount"].(float64)
	configuredNodeCount := int(configuredNode)
	nodesCount, _ := strconv.Atoi(nodes)
	if nodesCount > 0 {
		nodeDiff := nodesCount - configuredNodeCount

		if nodeDiff != 0 {
			return nil, fmt.Errorf("Node count is not matching with Service Template")
		}
	}

	deploymentPayload := map[string]interface{}{
		"deploymentName":        deploymentName,
		"deploymentDescription": deploymentDesc,
		"serviceTemplate":       templateData,
		"updateServerFirmware":  true,
		"firmwareRepositoryId":  firmwareRepositoryID, // TODO
	}

	deploymentPayloadJSON, _ := json.Marshal(deploymentPayload)
	req, httpError = http.NewRequest(http.MethodPost, gc.host+"/Api/V1/Deployment", bytes.NewBuffer(deploymentPayloadJSON))
	if httpError != nil {
		return nil, httpError
	}

	if gc.version == "4.0" {
		req.Header.Set("Authorization", "Bearer "+gc.token)

		err := setCookie(req.Header, gc.host)
		if err != nil {
			return nil, fmt.Errorf("Error While Handling Cookie: %s", err)
		}
	} else {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(gc.username+":"+gc.password)))
	}
	req.Header.Set("Content-Type", "application/json")

	client = gc.http
	httpResp, httpRespError = client.Do(req)
	if httpRespError != nil {
		return nil, httpRespError
	}

	responseString, ioErr := extractString(httpResp)
	if ioErr != nil {
		return nil, fmt.Errorf("Error Extracting Response: %s", ioErr)
	}

	if httpResp.StatusCode != 200 {
		var deploymentResponse types.ServiceFailedResponse
		parseError = json.Unmarshal([]byte(responseString), &deploymentResponse)
		if parseError != nil {
			return nil, fmt.Errorf("Error While Parsing Response Data For Deployment: %s", parseError)
		}

		deploymentResponse.StatusCode = 400
		return nil, fmt.Errorf("Error While Parsing Response Data For Deployment: %s", deploymentResponse.Messages[0].DisplayMessage)
	}

	var deploymentResponse types.ServiceResponse
	parseError = json.Unmarshal([]byte(responseString), &deploymentResponse)
	if parseError != nil {
		return nil, fmt.Errorf("Error While Parsing Response Data For Deployment: %s", parseError)
	}

	deploymentResponse.StatusCode = 200
	return &deploymentResponse, nil
}

// UpdateService updates an existing service in the ScaleIO Gateway.
func (gc *GatewayClient) UpdateService(deploymentID, deploymentName, deploymentDesc, nodes, nodename string) (*types.ServiceResponse, error) {
	defer TimeSpent("UpdateService", time.Now())

	path := fmt.Sprintf("/Api/V1/Deployment/%v", deploymentID)

	req, httpError := http.NewRequest(http.MethodGet, gc.host+path, nil)
	if httpError != nil {
		return nil, httpError
	}

	if gc.version == "4.0" {
		req.Header.Set("Authorization", "Bearer "+gc.token)

		err := setCookie(req.Header, gc.host)
		if err != nil {
			return nil, fmt.Errorf("Error While Handling Cookie: %s", err)
		}

	} else {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(gc.username+":"+gc.password)))
	}

	req.Header.Set("Content-Type", "application/json")

	client := gc.http
	httpResp, httpRespError := client.Do(req)
	if httpRespError != nil {
		return nil, httpRespError
	}

	responseString, _ := extractString(httpResp)

	if httpResp.StatusCode != 200 || responseString == "" {
		var deploymentResponse types.ServiceFailedResponse
		parseError := json.Unmarshal([]byte(responseString), &deploymentResponse)
		if parseError != nil {
			return nil, fmt.Errorf("Error While Parsing Response Data For Deployment: %s", parseError)
		}
		return nil, fmt.Errorf("Error While Parsing Response Data For Deployment: %s", deploymentResponse.Messages[0].DisplayMessage)
	}

	var deploymentResponse types.ServiceResponse
	parseError := json.Unmarshal([]byte(responseString), &deploymentResponse)
	if parseError != nil {
		return nil, fmt.Errorf("Error While Parsing Response Data For Deployment: %s", parseError)
	}

	var deploymentPayloadJSON []byte
	deployedNodes := deploymentResponse.ServiceTemplate.ServerCount
	nodesCount, _ := strconv.Atoi(nodes)
	nodeDiff := nodesCount - deployedNodes

	if nodeDiff >= 1 {
		var deploymentData map[string]interface{}

		parseError := json.Unmarshal([]byte(responseString), &deploymentData)
		if parseError != nil {
			return nil, fmt.Errorf("Error While Parsing Response Data For Deployment: %s", parseError)
		}

		deploymentData["deploymentName"] = deploymentName
		deploymentData["deploymentDescription"] = deploymentDesc

		// Access the "components" field
		serviceTemplate, ok := deploymentData["serviceTemplate"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("Error While Parsing Response Data For Deployment")
		}

		components, ok := serviceTemplate["components"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("Error While Parsing Response Data For Deployment")
		}

		// Find the component with type "SERVER"
		var serverComponent map[string]interface{}

		componentFound := false

		for _, comp := range components {
			comp := comp.(map[string]interface{})
			if comp["type"].(string) == "SERVER" && comp["name"].(string) == nodename {
				serverComponent = comp
				componentFound = true
				break
			}
		}

		if !componentFound {
			return nil, fmt.Errorf("Host to clone from not found")
		}

		for numberOfNode := 1; numberOfNode <= nodeDiff; numberOfNode++ {
			// Deep copy the component
			clonedComponent := make(map[string]interface{})
			for key, value := range serverComponent {
				clonedComponent[key] = value
			}

			uuid := uuid.New().String()

			// Modify ID and GUID of the cloned component
			clonedComponent["id"] = uuid
			clonedComponent["name"] = uuid
			clonedComponent["brownfield"] = false
			clonedComponent["identifier"] = nil
			clonedComponent["asmGUID"] = nil
			clonedComponent["puppetCertName"] = nil
			clonedComponent["osPuppetCertName"] = nil
			clonedComponent["managementIpAddress"] = nil

			// Deep copy resources
			resources, ok := clonedComponent["resources"].([]interface{})
			if !ok {
				return nil, fmt.Errorf("Error While Parsing Response Data For Deployment")
			}

			clonedResources := make([]interface{}, len(resources))
			for i, res := range resources {
				resCopy := make(map[string]interface{})
				for k, v := range res.(map[string]interface{}) {
					resCopy[k] = v
				}
				clonedResources[i] = resCopy
			}
			clonedComponent["resources"] = clonedResources

			// Exclude list of parameters to skip
			excludeList := map[string]bool{
				"razor_image":         true,
				"scaleio_enabled":     true,
				"scaleio_role":        true,
				"compression_enabled": true,
				"replication_enabled": true,
			}

			// Iterate over resources to modify parameters
			for _, comp := range clonedResources {
				comp := comp.(map[string]interface{})
				if comp["id"].(string) == "asm::server" {

					comp["guid"] = nil

					parameters, ok := comp["parameters"].([]interface{})
					if !ok {
						return nil, fmt.Errorf("Error While Parsing Response Data For Deployment")
					}

					clonedParams := make([]interface{}, len(parameters))
					for i, param := range parameters {
						paramCopy := make(map[string]interface{})
						for k, v := range param.(map[string]interface{}) {
							paramCopy[k] = v
						}
						clonedParams[i] = paramCopy
					}

					for _, parameter := range clonedParams {
						parameter := parameter.(map[string]interface{})
						if !excludeList[parameter["id"].(string)] {
							if parameter["id"].(string) == "scaleio_mdm_role" {
								parameter["guid"] = nil
								parameter["value"] = "standby_mdm"
							} else {
								parameter["guid"] = nil
								parameter["value"] = nil
							}
						}
					}

					// Update parameters in the component
					comp["parameters"] = clonedParams
				}
			}

			// Append the cloned component back to the components array
			components = append(components, clonedComponent)
			// Update serviceTemplate with modified components
			serviceTemplate["components"] = components
		}

		// Update deploymentData with modified serviceTemplate
		deploymentData["serviceTemplate"] = serviceTemplate
		// Update other fields as needed
		deploymentData["scaleUp"] = true
		deploymentData["retry"] = true
		// Marshal deploymentData to JSON
		deploymentPayloadJSON, _ = json.Marshal(deploymentData)

	} else if nodeDiff == 0 {

		deploymentResponse, jsonParseError := jsonToMap(responseString)
		if jsonParseError != nil {
			return nil, jsonParseError
		}

		deploymentResponse["deploymentName"] = deploymentName
		deploymentResponse["deploymentDescription"] = deploymentDesc
		deploymentPayloadJSON, _ = json.Marshal(deploymentResponse)
	} else if nodeDiff < 0 {
		return nil, fmt.Errorf("Removing node(s) is not supported")
	}

	req, httpError = http.NewRequest("PUT", gc.host+"/Api/V1/Deployment/"+deploymentID, bytes.NewBuffer(deploymentPayloadJSON))
	if httpError != nil {
		return nil, httpError
	}

	if gc.version == "4.0" {
		req.Header.Set("Authorization", "Bearer "+gc.token)

		err := setCookie(req.Header, gc.host)
		if err != nil {
			return nil, fmt.Errorf("Error While Handling Cookie: %s", err)
		}
	} else {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(gc.username+":"+gc.password)))
	}
	req.Header.Set("Content-Type", "application/json")

	client = gc.http
	httpResp, httpRespError = client.Do(req)
	if httpRespError != nil {
		return nil, httpRespError
	}

	responseString, ioErr := extractString(httpResp)
	if ioErr != nil {
		return nil, fmt.Errorf("Error Extracting Response: %s", ioErr)
	}

	if httpResp.StatusCode != http.StatusOK {
		var deploymentResponse types.ServiceFailedResponse
		parseError := json.Unmarshal([]byte(responseString), &deploymentResponse)
		if parseError != nil {
			return nil, fmt.Errorf("Error While Parsing Response Data For Deployment: %s", parseError)
		}
		deploymentResponse.StatusCode = 400
		return nil, fmt.Errorf("Error While Parsing Response Data For Deployment: %s", deploymentResponse.Messages[0].DisplayMessage)
	}

	deploymentResponse = types.ServiceResponse{}
	parseError = json.Unmarshal([]byte(responseString), &deploymentResponse)
	if parseError != nil {
		return nil, fmt.Errorf("Error While Parsing Response Data For Deployment: %s", parseError)
	}
	deploymentResponse.StatusCode = 200
	return &deploymentResponse, nil
}

// GetServiceDetailsByID retrieves service details by deployment ID.
func (gc *GatewayClient) GetServiceDetailsByID(deploymentID string, newToken bool) (*types.ServiceResponse, error) {
	defer TimeSpent("GetServiceDetailsByID", time.Now())

	if newToken {
		bodyData := map[string]interface{}{
			"username": gc.username,
			"password": gc.password,
		}

		body, _ := json.Marshal(bodyData)

		req, err := http.NewRequest(http.MethodPost, gc.host+"/rest/auth/login", bytes.NewBuffer(body))
		if err != nil {
			return nil, err
		}

		req.Header.Add("Content-Type", "application/json")

		resp, err := gc.http.Do(req)
		if err != nil {
			return nil, err
		}

		defer func() {
			if err := resp.Body.Close(); err != nil {
				doLog(logger.Error, err.Error())
			}
		}()

		// parse the response
		switch {
		case resp == nil:
			return nil, errNilReponse
		case !(resp.StatusCode >= 200 && resp.StatusCode <= 299):
			return nil, ParseJSONError(resp)
		}

		bs, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		responseBody := string(bs)
		result := make(map[string]interface{})
		jsonErr := json.Unmarshal([]byte(responseBody), &result)
		if err != nil {
			return nil, fmt.Errorf("Error For Uploading Package: %s", jsonErr)
		}

		token := result["access_token"].(string)
		gc.token = token
	}

	path := fmt.Sprintf("/Api/V1/Deployment/%v", deploymentID)

	req, httpError := http.NewRequest(http.MethodGet, gc.host+path, nil)
	if httpError != nil {
		return nil, httpError
	}

	if gc.version == "4.0" {
		req.Header.Set("Authorization", "Bearer "+gc.token)

		err := setCookie(req.Header, gc.host)
		if err != nil {
			return nil, fmt.Errorf("Error While Handling Cookie: %s", err)
		}

	} else {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(gc.username+":"+gc.password)))
	}

	req.Header.Set("Content-Type", "application/json")

	client := gc.http
	httpResp, httpRespError := client.Do(req)
	if httpRespError != nil {
		return nil, httpRespError
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Couldn't find service with the given filter")
	}

	var deploymentResponse types.ServiceResponse
	responseString, _ := extractString(httpResp)
	parseError := json.Unmarshal([]byte(responseString), &deploymentResponse)
	if parseError != nil {
		return nil, fmt.Errorf("Error While Parsing Response Data For Deployment: %s", parseError)
	}
	return &deploymentResponse, nil
}

// GetServiceDetailsByFilter retrieves service details based on a filter and value.
func (gc *GatewayClient) GetServiceDetailsByFilter(filter, value string) ([]types.ServiceResponse, error) {
	defer TimeSpent("GetServiceDetailsByFilter", time.Now())

	encodedValue := url.QueryEscape(value)
	path := fmt.Sprintf("/Api/V1/Deployment?filter=eq,%v,%v", filter, encodedValue)

	req, httpError := http.NewRequest(http.MethodGet, gc.host+path, nil)
	if httpError != nil {
		return nil, httpError
	}

	if gc.version == "4.0" {
		req.Header.Set("Authorization", "Bearer "+gc.token)

		err := setCookie(req.Header, gc.host)
		if err != nil {
			return nil, fmt.Errorf("Error While Handling Cookie: %s", err)
		}

	} else {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(gc.username+":"+gc.password)))
	}

	req.Header.Set("Content-Type", "application/json")

	client := gc.http
	httpResp, httpRespError := client.Do(req)
	if httpRespError != nil {
		return nil, httpRespError
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Couldn't find service with the given filter")
	}

	var deploymentResponse []types.ServiceResponse
	responseString, _ := extractString(httpResp)
	parseError := json.Unmarshal([]byte(responseString), &deploymentResponse)
	if parseError != nil {
		return nil, fmt.Errorf("Error While Parsing Response Data For Deployment: %s", parseError)
	}
	if len(deploymentResponse) == 0 {
		return nil, fmt.Errorf("Couldn't find service with the given filter")
	}

	return deploymentResponse, nil
}

// GetAllServiceDetails retrieves all service details from the GatewayClient.
func (gc *GatewayClient) GetAllServiceDetails() ([]types.ServiceResponse, error) {
	defer TimeSpent("DeploGetServiceDetailsByIDyService", time.Now())

	req, httpError := http.NewRequest(http.MethodGet, gc.host+"/Api/V1/Deployment/", nil)
	if httpError != nil {
		return nil, httpError
	}

	if gc.version == "4.0" {
		req.Header.Set("Authorization", "Bearer "+gc.token)

		err := setCookie(req.Header, gc.host)
		if err != nil {
			return nil, fmt.Errorf("Error While Handling Cookie: %s", err)
		}
	} else {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(gc.username+":"+gc.password)))
	}

	req.Header.Set("Content-Type", "application/json")

	client := gc.http
	httpResp, httpRespError := client.Do(req)
	if httpRespError != nil {
		return nil, httpRespError
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Couldn't find service with the given filter")
	}

	var deploymentResponse []types.ServiceResponse
	responseString, _ := extractString(httpResp)
	parseError := json.Unmarshal([]byte(responseString), &deploymentResponse)
	if parseError != nil {
		return nil, fmt.Errorf("Error While Parsing Response Data For Deployment: %s", parseError)
	}
	return deploymentResponse, nil
}

// DeleteService deletes a service by its ID, along with servers in inventory and managed state.
func (gc *GatewayClient) DeleteService(serviceID, serversInInventory, serversManagedState string) (*types.ServiceResponse, error) {
	var deploymentResponse types.ServiceResponse

	deploymentResponse.StatusCode = 400

	defer TimeSpent("DeleteService", time.Now())

	req, httpError := http.NewRequest("DELETE", gc.host+"/Api/V1/Deployment/"+serviceID+"?serversInInventory="+serversInInventory+"&serversManagedState="+serversManagedState, nil)
	if httpError != nil {
		return nil, httpError
	}

	if gc.version == "4.0" {
		req.Header.Set("Authorization", "Bearer "+gc.token)

		err := setCookie(req.Header, gc.host)
		if err != nil {
			return nil, fmt.Errorf("Error While Handling Cookie: %s", err)
		}
	} else {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(gc.username+":"+gc.password)))
	}

	req.Header.Set("Content-Type", "application/json")

	client := gc.http
	httpResp, httpRespError := client.Do(req)
	if httpRespError != nil {
		return nil, httpRespError
	}

	if httpResp.StatusCode == 204 {
		deploymentResponse.StatusCode = 200
		return &deploymentResponse, nil
	}
	return nil, fmt.Errorf("Couldn't delete service")
}

// GetServiceComplianceDetails retrieves service compliance details for a given deployment.
func (gc *GatewayClient) GetServiceComplianceDetails(deploymentID string) ([]types.ComplianceReport, error) {
	defer TimeSpent("GetServiceComplianceDetails", time.Now())

	path := fmt.Sprintf("/Api/V1/Deployment/%v/firmware/compliancereport", deploymentID)

	req, httpError := http.NewRequest(http.MethodGet, gc.host+path, nil)
	if httpError != nil {
		return nil, httpError
	}

	if gc.version == "4.0" {
		req.Header.Set("Authorization", "Bearer "+gc.token)

		err := setCookie(req.Header, gc.host)
		if err != nil {
			return nil, fmt.Errorf("Error while handling cookie: %s", err)
		}

	} else {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(gc.username+":"+gc.password)))
	}

	req.Header.Set("Content-Type", "application/json")

	client := gc.http
	httpResp, httpRespError := client.Do(req)
	if httpRespError != nil {
		return nil, httpRespError
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Couldn't find compliance report for given deployment")
	}

	var complianceReports []types.ComplianceReport
	responseString, _ := extractString(httpResp)
	parseError := json.Unmarshal([]byte(responseString), &complianceReports)
	if parseError != nil {
		return nil, fmt.Errorf("Error while parsing response data for compliance report: %s", parseError)
	}
	if len(complianceReports) == 0 {
		return nil, fmt.Errorf("Couldn't find compliance report for given deployment")
	}

	return complianceReports, nil
}

// GetServiceComplianceDetailsByFilter retrieves service compliance details based on a filter and value.
func (gc *GatewayClient) GetServiceComplianceDetailsByFilter(deploymentID, filter, value string) ([]types.ComplianceReport, error) {
	defer TimeSpent("GetServiceComplianceDetailsByFilter", time.Now())

	complianceReports, err := gc.GetServiceComplianceDetails(deploymentID)
	if err != nil || len(complianceReports) == 0 {
		return nil, fmt.Errorf("Couldn't find compliance report for the given deployment")
	}

	filteredComplianceReports := make([]types.ComplianceReport, 0)
	for _, complianceReport := range complianceReports {
		switch filter {
		case "IpAddress":
			if complianceReport.IPAddress == value {
				filteredComplianceReports = append(filteredComplianceReports, complianceReport)
			}
		case "ServiceTag":
			if complianceReport.ServiceTag == value {
				filteredComplianceReports = append(filteredComplianceReports, complianceReport)
			}
		case "Compliant":
			if strconv.FormatBool(complianceReport.Compliant) == value {
				filteredComplianceReports = append(filteredComplianceReports, complianceReport)
			}
		case "HostName":
			if complianceReport.HostName == value {
				filteredComplianceReports = append(filteredComplianceReports, complianceReport)
			}
		case "ID":
			if complianceReport.ID == value {
				filteredComplianceReports = append(filteredComplianceReports, complianceReport)
			}
		default:
			return nil, fmt.Errorf("Invalid filter provided")
		}
	}

	return filteredComplianceReports, nil
}
