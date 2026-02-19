package vantara

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

const requiredMajorVersion = 1
const requiredMinorVersion = 9

// extractIPAddress extracts an IP address from a URL string
func extractIPAddress(url string) (string, error) {
	ipRegex := `\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`
	r := regexp.MustCompile(ipRegex)
	match := r.FindString(url)
	if match == "" {
		return "", errors.New("IP address not found")
	}
	return match, nil
}

type BlockStorageAPI struct {
	GumIPAddr  string
	Port       string
	StorageID  string
	BaseURL    string
	ObjectURL  string
	ServiceURL string

	// Session management
	httpClient       *http.Client
	sessionToken     string
	sessionId        string
	sessionStartTime time.Time
	username         string
	password         string
	isConnected      bool
}

func NewBlockStorageAPI(gumIPAddr, port, storageID, username, password string) *BlockStorageAPI {
	baseURL := fmt.Sprintf("https://%s:%s/ConfigurationManager/v1", gumIPAddr, port)
	return &BlockStorageAPI{
		GumIPAddr:  gumIPAddr,
		Port:       port,
		StorageID:  storageID,
		BaseURL:    baseURL,
		ObjectURL:  "/objects",
		ServiceURL: "/services",
		username:   username,
		password:   password,
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			Timeout: 30 * time.Second,
		},
		isConnected: false,
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

// makeHTTPRequest performs an HTTP request using the reusable HTTP client
func (api *BlockStorageAPI) makeHTTPRequest(methodType, url string, body, headers map[string]string) (map[string]interface{}, error) {
	klog.V(2).Infof("HTTP %s %s", methodType, url)

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

	resp, err := api.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	klog.V(2).Infof("Response status: %s", resp.Status)

	// Check for 503 status code and retry
	if resp.StatusCode == http.StatusServiceUnavailable {
		klog.Warning("Service unavailable (503), retrying after 60s...")
		time.Sleep(60 * time.Second)
		return api.makeHTTPRequest(methodType, url, body, headers)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}
	return result, nil
}

func (api *BlockStorageAPI) checkUpdate(jobID string, headers map[string]string) (map[string]interface{}, error) {
	url := api.Job(jobID)
	return api.makeHTTPRequest("GET", url, nil, headers)
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
	result, err := api.makeHTTPRequest(methodType, url, body, headers)
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

// Connect establishes a session with the Vantara storage API
func (api *BlockStorageAPI) Connect() error {
	if api.isConnected {
		// Check if session is still valid (< 25 minutes old to avoid timeout at 30 minutes)
		if time.Since(api.sessionStartTime) < 25*time.Minute {
			return nil // Reuse existing session
		}
		// Session expired, disconnect and reconnect
		api.Disconnect()
	}

	base64Auth := base64.StdEncoding.EncodeToString([]byte(api.username + ":" + api.password))

	headers := map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json",
		"Response-Job-Status": "Completed",
		"Authorization":       "Basic " + base64Auth,
	}
	// Check API version
	url := api.APIVersion()
	klog.Infof("Connecting to Vantara storage at %s", url)
	r, err := api.makeHTTPRequest("GET", url, nil, headers)
	if err != nil {
		return fmt.Errorf("failed to get API version: %w", err)
	}
	apiVersion := r["apiVersion"].(string)
	if err := CheckAPIVersion(apiVersion, requiredMajorVersion, requiredMinorVersion); err != nil {
		return err
	}
	klog.Infof("API version: %s", apiVersion)

	// Generate session
	url = api.GenerateSession()
	r, err = api.makeHTTPRequest("POST", url, map[string]string{}, headers)
	if err != nil {
		return fmt.Errorf("failed to generate session: %w", err)
	}

	api.sessionToken = r["token"].(string)
	sessionIdFloat64 := r["sessionId"].(float64)
	api.sessionId = fmt.Sprintf("%d", int(sessionIdFloat64))
	api.sessionStartTime = time.Now()
	api.isConnected = true

	klog.Infof("Vantara session established: %s", api.sessionId)
	return nil
}

// Disconnect closes the current session
func (api *BlockStorageAPI) Disconnect() error {
	if !api.isConnected {
		return nil
	}

	headers := api.sessionHeaders()
	url := api.DiscardSession(api.sessionId)
	_, err := api.makeHTTPRequest("DELETE", url, nil, headers)
	if err != nil {
		klog.Warningf("Failed to discard session: %v", err)
		// Don't return error - we're cleaning up anyway
	}

	api.isConnected = false
	api.sessionToken = ""
	api.sessionId = ""
	klog.Info("Vantara session discarded")
	return nil
}

// GetLdev retrieves LDEV information
func (api *BlockStorageAPI) GetLdev(ldevId string) (*LdevResponse, error) {
	if err := api.ensureConnected(); err != nil {
		return nil, err
	}

	url := api.Ldev(ldevId)
	headers := api.sessionHeaders()

	r, err := api.makeHTTPRequest("GET", url, nil, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to get LDEV %s: %w", ldevId, err)
	}

	// Convert map to typed struct
	var ldev LdevResponse
	jsonBytes, _ := json.Marshal(r)
	if err := json.Unmarshal(jsonBytes, &ldev); err != nil {
		return nil, fmt.Errorf("failed to parse LDEV response: %w", err)
	}

	return &ldev, nil
}

// AddPath adds a path mapping for a LUN
func (api *BlockStorageAPI) AddPath(ldevId string, portId string, hostGroupNumber string) error {
	if err := api.ensureConnected(); err != nil {
		return err
	}

	url := api.Luns()
	headers := api.sessionHeaders()
	headers["Response-Job-Status"] = "Completed"

	body := map[string]string{
		"ldevId":          ldevId,
		"portId":          portId,
		"hostGroupNumber": hostGroupNumber,
	}

	bodyJson, _ := json.Marshal(body)
	klog.V(2).Infof("AddPath request body: %s", string(bodyJson))

	_, err := api.InvokeAsyncCommand("POST", url, body, headers)
	if err != nil {
		return fmt.Errorf("failed to add path for LDEV %s: %w", ldevId, err)
	}

	return nil
}

// DeletePath removes a path mapping for a LUN
func (api *BlockStorageAPI) DeletePath(ldevId string, portId string, hostGroupNumber string, lunId string) error {
	if err := api.ensureConnected(); err != nil {
		return err
	}

	objectID := fmt.Sprintf("%s,%s,%s", portId, hostGroupNumber, lunId)
	url := api.Lun(objectID)
	headers := api.sessionHeaders()
	headers["Response-Job-Status"] = "Completed"

	_, err := api.InvokeAsyncCommand("DELETE", url, map[string]string{}, headers)
	if err != nil {
		return fmt.Errorf("failed to delete path %s for LDEV %s: %w", objectID, ldevId, err)
	}

	return nil
}

// GetPortDetails retrieves port login details
func (api *BlockStorageAPI) GetPortDetails() (*PortDetailsResponse, error) {
	if err := api.ensureConnected(); err != nil {
		return nil, err
	}

	url := api.Ports() + "?detailInfoType=logins"
	headers := api.sessionHeaders()

	r, err := api.makeHTTPRequest("GET", url, nil, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to get port details: %w", err)
	}

	var portDetails PortDetailsResponse
	jsonBytes, _ := json.Marshal(r)
	if err := json.Unmarshal(jsonBytes, &portDetails); err != nil {
		return nil, fmt.Errorf("failed to parse port details: %w", err)
	}

	return &portDetails, nil
}

// ensureConnected checks the connection status and reconnects if needed
func (api *BlockStorageAPI) ensureConnected() error {
	if !api.isConnected {
		return api.Connect()
	}

	// Check session age and refresh if needed
	if time.Since(api.sessionStartTime) > 25*time.Minute {
		klog.Info("Session approaching expiration, reconnecting...")
		api.Disconnect()
		return api.Connect()
	}

	return nil
}

// sessionHeaders returns common headers for session-authenticated requests
func (api *BlockStorageAPI) sessionHeaders() map[string]string {
	return map[string]string{
		"Content-Type":  "application/json",
		"Accept":        "application/json",
		"Authorization": "Session " + api.sessionToken,
	}
}
