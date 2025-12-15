package vantara

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

type BlockStorageAPI struct {
	GumIPAddr  string
	Port       string
	StorageID  string
	BaseURL    string
	ObjectURL  string
	ServiceURL string
}

func NewBlockStorageAPI(gumIPAddr, port, storageID string) *BlockStorageAPI {
	baseURL := fmt.Sprintf("https://%s:%s/ConfigurationManager/v1", gumIPAddr, port)
	return &BlockStorageAPI{
		GumIPAddr:  gumIPAddr,
		Port:       port,
		StorageID:  storageID,
		BaseURL:    baseURL,
		ObjectURL:  "/objects",
		ServiceURL: "/services",
	}
}

func (api *BlockStorageAPI) GetStorageID() string {
	return api.StorageID
}

func (api *BlockStorageAPI) Ldevs() string {
	return api.BaseURL + api.ObjectURL + "/ldevs"
}

func (api *BlockStorageAPI) Ldev(objectID string) string {
	return api.Ldevs() + "/" + objectID
}

func (api *BlockStorageAPI) GenerateSession() string {
	return api.BaseURL + api.ObjectURL + "/sessions"
}

func (api *BlockStorageAPI) DiscardSession(objectID string) string {
	return api.BaseURL + api.ObjectURL + "/sessions/" + objectID
}

func (api *BlockStorageAPI) Job(objectID string) string {
	return api.BaseURL + api.ObjectURL + "/jobs/" + objectID
}

func (api *BlockStorageAPI) Ports() string {
	return api.BaseURL + api.ObjectURL + "/ports"
}
func (api *BlockStorageAPI) Luns() string {
	return api.BaseURL + api.ObjectURL + "/luns"
}

func (api *BlockStorageAPI) Lun(objectID string) string {
	return api.Luns() + "/" + objectID
}

func (api *BlockStorageAPI) APIVersion() string {
	return fmt.Sprintf("https://%s:%s/ConfigurationManager/configuration/version", api.GumIPAddr, api.Port)
}

func MakeHTTPRequest(methodType, url string, body, headers map[string]string, authType, authValue string) (map[string]interface{}, error) {
	klog.Infof("Making HTTP request:")
	klog.Infof("Method: %s", methodType)
	klog.Infof("URL: %s", url)
	klog.Infof("Headers: %v", headers)
	klog.Infof("Auth Type: %s", authType)

	// Disable TLS certificate verification
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Create request body
	var reqBody io.Reader
	if body != nil { // If body is not nil, encode it as JSON
		jsonBody, err := json.Marshal(body)
		if err != nil {
			klog.Errorf("Error encoding JSON: %v", err)
			return nil, err
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(methodType, url, reqBody)
	if err != nil {
		klog.Errorf("Error creating request: %v", err)
		return nil, err
	}

	// Set headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Set authentication
	if authType == "basic" {
		// authValue should be "username:password"
		base64Auth := base64.StdEncoding.EncodeToString([]byte(authValue))
		req.Header.Set("Authorization", "Basic "+base64Auth)
	} else if authType == "session" {
		// authValue should be the token
		req.Header.Set("Authorization", authValue)
	}

	resp, err := client.Do(req)
	if err != nil {
		klog.Errorf("Error making request: %v", err)
		return nil, err
	}
	klog.Infof("Response status: %s", resp.Status)
	defer resp.Body.Close()

	// Todo: Check for 503 status code and retry
	if resp.StatusCode == http.StatusServiceUnavailable {
		resp.Body.Close()
		klog.Errorf("Service unavailable, retrying...")
		time.Sleep(60 * time.Second)
		return MakeHTTPRequest(methodType, url, body, headers, authType, authValue)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		klog.Errorf("Request failed with status code: %d", resp.StatusCode)
		return nil, fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

func (api *BlockStorageAPI) checkUpdate(jobID string, headers map[string]string) (map[string]interface{}, error) {
	url := api.Job(jobID)
	return MakeHTTPRequest("GET", url, nil, headers, "", "")

}

func CheckAPIVersion(apiVersion string, requiredMajorVersion, requiredMinorVersion int) error {
	versionParts := strings.Split(apiVersion, ".")
	if len(versionParts) < 2 {
		return fmt.Errorf("invalid API version format: %s", apiVersion)
	}

	majorVersion, err := strconv.Atoi(versionParts[0])
	if err != nil {
		return fmt.Errorf("invalid major version: %s", versionParts[0])
	}

	minorVersion, err := strconv.Atoi(versionParts[1])
	if err != nil {
		return fmt.Errorf("invalid minor version: %s", versionParts[1])
	}

	if !((majorVersion == requiredMajorVersion && minorVersion >= requiredMinorVersion) || majorVersion >= requiredMajorVersion+1) {
		return fmt.Errorf("this program requires API Version %d.%d.x or newer", requiredMajorVersion, requiredMinorVersion)
	}

	return nil
}

func (api *BlockStorageAPI) InvokeAsyncCommand(methodType, url string, body, headers map[string]string) (string, error) {

	result, err := MakeHTTPRequest(methodType, url, body, headers, "session", headers["Authorization"])
	if err != nil {
		return "", err
	}
	klog.Infof("Request was accepted. JOB URL: %v", result["self"])

	status := "Initializing"
	retryCount := 1
	waitTime := 1 // FIRST_WAIT_TIME

	for status != "Completed" {
		if retryCount > 10 { // MAX_RETRY_COUNT
			return "", fmt.Errorf("timeout error: operation was not completed")
		}
		time.Sleep(time.Duration(waitTime) * time.Second)

		jobResult, err := api.checkUpdate(fmt.Sprintf("%d", int(result["jobId"].(float64))), headers)
		if err != nil {
			klog.Errorf("Error checking job status: %v", err)
			return "", err
		}
		status = jobResult["status"].(string)
		klog.Infof("Status: %s", status)
		if waitTime*2 < 120 {
			waitTime *= 2
		} else {
			waitTime = 120
		}
		retryCount++
	}

	klog.Infof("Async job was succeeded. status: %s", status)
	return status, nil
}
