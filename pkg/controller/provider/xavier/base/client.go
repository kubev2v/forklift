package base

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/virt-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/virt-controller/pkg/settings"
	"io/ioutil"
	"net/http"
	liburl "net/url"
	"strconv"
	"strings"
	"time"
)

//
// Body object fields.
const (
	Provider        = "provider"
	DataCenter      = "datacenter"
	Cluster         = "cluster"
	VmName          = "vmName"
	DiskSpace       = "diskSpace"
	Memory          = "memory"
	CpuCores        = "cpuCores"
	GuestOs         = "guestOSFullName"
	OsProduct       = "osProductName"
	Product         = "product"
	Version         = "version"
	HostName        = "host_name"
	BalloonedMemory = "balloonedMemory"
	HasMemoryHotAdd = "hasMemoryHotAdd"
	HasCpuHotAdd    = "hasCpuHotAdd"
	HasCpuHotRemove = "hasCpuHotRemove"
	HasCpuAffinity  = "cpuAffinityNotNull"
	HasDrsEnabled   = "hasVmDrsConfig"
	HasHaEnabled    = "hasVmHaConfig"
	HasPassthrough  = "hasPassthroughDevice"
	HasUsb          = "hasUSBcontrollers"
	HasRdmDisk      = "hasRdmDisk"
	ScanDate        = "scanRunDate"
)

//
// Application settings.
var Settings = &settings.Settings

type List = []interface{}

type Object = map[string]interface{}

//
// Model adapter.
type ModelAdapter interface {
	// Update concerns.
	UpdateConcerns(model libmodel.Model, concerns []string)
	// Update the xavier body.
	UpdateBody(object Object, model libmodel.Model) error
}

//
// Factory.
func New(db libmodel.DB, adapter ModelAdapter) *Client {
	return &Client{
		DB:       db,
		URL:      Settings.Inventory.Xavier.URL,
		User:     Settings.Inventory.Xavier.User,
		Password: Settings.Inventory.Xavier.Password,
		Adapter:  adapter,
	}
}

//
// Xavier client.
type Client struct {
	// Database.
	libmodel.DB
	// Service URL.
	URL string
	// User.
	User string
	// Password
	Password string
	// Builder
	Adapter ModelAdapter
}

//
// Analyze the specified VM.
func (r *Client) Analyze(model libmodel.Model) (err error) {
	concerns := []string{}
	if r.URL == "" {
		r.Adapter.UpdateConcerns(model, concerns)
		Log.Info("Xavier: URL not configured.")
		return
	}
	if mX, cast := model.(interface{ Analyzed() bool }); cast {
		if mX.Analyzed() {
			return
		}
	}
	reply := Object{}
	body, err := r.body(model)
	if err != nil {
		err = liberr.Wrap(err)
	}
	err = r.post(&body, &reply)
	if err != nil {
		return
	}
	report, err := r.find(
		reply,
		"result",
		"execution-results",
		"results",
		"2",
		"value",
		"org.drools.core.runtime.rule.impl.FlatQueryResults",
		"idResultMaps",
		"element",
		"0",
		"element",
		"0",
		"value",
		"org.jboss.xavier.analytics.pojo.output.workload.inventory.WorkloadInventoryReportModel")
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	providers, err := r.find(report, "recommendedTargetsIMS")
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	flags, err := r.find(report, "flagsIMS")
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if list, cast := flags.(List); cast {
		for _, flag := range list {
			concerns = append(concerns, flag.(string))
		}
	}
	if len(concerns) == 0 {
		recommended := false
		if list, cast := providers.(List); cast {
			for _, provider := range list {
				if provider == "OCP" {
					recommended = true
					break
				}
			}
		}
		if !recommended {
			concerns = append(
				concerns,
				"CNV not recommended.")
		}
	}
	r.Adapter.UpdateConcerns(model, concerns)

	Log.Info("Analyzed", "vm", model.String())

	return
}

//
// Build body
func (r *Client) body(model libmodel.Model) (body Object, err error) {
	about, _ := r.about()
	object := Object{
		Provider:        "",
		DataCenter:      "",
		Cluster:         "",
		VmName:          "",
		DiskSpace:       0,
		Memory:          0,
		CpuCores:        0,
		GuestOs:         "",
		OsProduct:       "",
		Product:         about.Product,
		Version:         about.APIVersion,
		HostName:        "",
		HasMemoryHotAdd: false,
		HasCpuHotAdd:    false,
		HasCpuHotRemove: false,
		HasCpuAffinity:  false,
		HasDrsEnabled:   false,
		HasHaEnabled:    false,
		HasPassthrough:  false,
		HasUsb:          false,
		HasRdmDisk:      false,
		BalloonedMemory: 0,
		ScanDate:        time.Now().Unix(),
	}
	insert := Object{
		"object": Object{
			"org.jboss.xavier.analytics.pojo.input.workload.inventory.VMWorkloadInventoryModel": object,
		},
		"out-identifier": "input",
	}
	body = Object{
		"lookup": "WorkloadInventoryKSession0",
		"commands": List{
			Object{
				"insert": insert,
			},
			Object{
				"fire-all-rules": Object{
					"out-identifier": "firedActivations",
				},
			},
			Object{
				"query": Object{
					"name":           "GetWorkloadInventoryReports",
					"out-identifier": "WorkloadInventoryReports",
					"arguments":      List{},
				},
			},
		},
	}
	err = r.Adapter.UpdateBody(object, model)
	if err != nil {
		err = liberr.Wrap(err)
	}

	return
}

//
// Add headers.
func (r Client) header() (header http.Header) {
	header = http.Header{}
	header.Add("Content-Type", "application/json")
	encoded := base64.StdEncoding.EncodeToString(
		[]byte(strings.Join([]string{
			r.User,
			r.Password,
		}, ":")))
	header.Add(
		"Authorization",
		"Basic "+encoded)

	return
}

//
// Post validation request.
func (r *Client) post(in, out *Object) (err error) {
	parsedURL, err := liburl.Parse(r.URL)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	body, _ := json.Marshal(in)
	reader := bytes.NewReader(body)
	request := &http.Request{
		Method: http.MethodPost,
		Body:   ioutil.NopCloser(reader),
		URL:    parsedURL,
		Header: r.header(),
	}
	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	status := response.StatusCode
	content := []byte{}
	if status == http.StatusOK {
		defer response.Body.Close()
		content, err = ioutil.ReadAll(response.Body)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		err = json.Unmarshal(content, out)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	} else {
		err = liberr.New(http.StatusText(status))
	}

	return
}

//
// Get object at path.
func (r Client) find(in interface{}, path ...string) (out interface{}, err error) {
	object := in
	for _, key := range path {
		switch object.(type) {
		case List:
			index, nErr := strconv.Atoi(key)
			if nErr != nil {
				err = liberr.Wrap(nErr)
				return
			}
			list := object.(List)
			if index < len(list) {
				object = list[index]
			} else {
				err = liberr.New("path not valid")
				return
			}
		case Object:
			found := false
			object, found = object.(Object)[key]
			if !found {
				err = liberr.New("path not valid")
				return
			}
		case string, int:
			return
		default:
			err = liberr.New("path not valid")
			return
		}
	}

	out = object

	return
}

//
// About vSphere.
func (r *Client) about() (about *vsphere.About, err error) {
	about = &vsphere.About{}
	err = r.DB.Get(about)
	if err != nil {
		liberr.Wrap(err)
	}

	return
}
