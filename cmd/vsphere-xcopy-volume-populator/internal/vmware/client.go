package vmware

import (
	"context"
	"encoding/xml"
	"reflect"
	"strings"

	"fmt"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/cli/esx"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"k8s.io/klog/v2"
)

//go:generate mockgen -destination=mocks/vmware_mock_client.go -package=vmware_mocks . Client
type Client interface {
	GetEsxByVm(ctx context.Context, vmName string) (*object.HostSystem, error)
	RunEsxCommand(ctx context.Context, host *object.HostSystem, command []string) ([]esx.Values, error)
	GetDatastore(ctx context.Context, datastore string) (*object.Datastore, error)
}

type VSphereClient struct {
	*govmomi.Client
}

func NewClient(hostname, username, password string) (Client, error) {
	ctx := context.Background()
	vcenterUrl := fmt.Sprintf("https://%s:%s@%s/sdk", username, password, hostname)
	u, err := soap.ParseURL(vcenterUrl)
	if err != nil {
		return nil, fmt.Errorf("Failed parsing vCenter URL: %w", err)
	}

	c, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		return nil, fmt.Errorf("Failed creating vSphere client: %w", err)
	}

	return &VSphereClient{Client: c}, nil
}

func (c *VSphereClient) RunEsxCommand(ctx context.Context, host *object.HostSystem, command []string) ([]esx.Values, error) {
	executor, err := esx.NewExecutor(ctx, c.Client.Client, host.Reference())
	if err != nil {
		return nil, err
	}

	// Invoke esxcli command
	klog.Infof("about to run esxcli command %s", command)
	res, err := executor.Run(ctx, command)
	if err != nil {
		klog.Errorf("Failed to run esxcli command: %+v %s %s", res, err, reflect.TypeOf(err))
		if fault, ok := err.(*esx.Fault); ok {
			fmt.Printf("CLI Fault: %+v\n", fault.MessageDetail())
		}

		return nil, err
	}
	for _, valueMap := range res.Values {
		message, _ := valueMap["message"]
		status, statusExists := valueMap["status"]
		klog.Infof("esxcli result message %s, status %v", message, status)
		if statusExists && strings.Join(status, "") != "0" {
			return nil, fmt.Errorf("Failed to invoke vmkfstools: %v", message)
		}
	}
	return res.Values, nil
}

func (c *VSphereClient) GetEsxByVm(ctx context.Context, vmName string) (*object.HostSystem, error) {
	finder := find.NewFinder(c.Client.Client, true)

	//FIXME - need to trace the VM by the datastore, which we should have because
	// of the vmdkPath and then run a finder of vms on a datastore.
	// Get the default datacenter
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		klog.Errorf("Failed to find default datacenter: %s", err)
		return nil, err
	}
	finder.SetDatacenter(dc)

	// Find the virtual machine by name
	vm, err := finder.VirtualMachine(ctx, vmName)
	if err != nil {
		return nil, fmt.Errorf("Failed to find VM %s: %v", vmName, err)
	}

	// Retrieve VM properties to get its host
	var vmProps mo.VirtualMachine
	err = vm.Properties(ctx, vm.Reference(), []string{"runtime.host"}, &vmProps)
	if err != nil {
		return nil, fmt.Errorf("Failed to get VM properties: %v", err)
	}

	hostRef := vmProps.Runtime.Host
	// Find host system
	host := object.NewHostSystem(c.Client.Client, *hostRef) // Adjust host query as needed
	if host == nil {
		klog.Error("Failed to find host:", err)
		return nil, err
	}
	return host, nil
}

func (c *VSphereClient) GetDatastore(ctx context.Context, datastore string) (*object.Datastore, error) {
	finder := find.NewFinder(c.Client.Client, true)
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		klog.Errorf("Failed to find default datacenter: %s", err)
		return nil, err
	}
	finder.SetDatacenter(dc)

	// Find the virtual machine by name
	ds, err := finder.Datastore(ctx, datastore)
	if err != nil {
		return nil, fmt.Errorf("Failed to find datastore %s: %v", datastore, err)
	}

	return ds, nil
}

type Obj struct {
	XMLName          xml.Name `xml:"urn:vim25 obj"`
	VersionID        string   `xml:"versionId,attr"`
	Type             string   `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	Fault            Fault    `xml:"fault"`
	LocalizedMessage string   `xml:"localizedMessage"`
}

type Fault struct {
	Type    string   `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	ErrMsgs []string `xml:"errMsg"`
}

func ErrToFault(err error) (*Fault, error) {
	f, ok := err.(*esx.Fault)
	if ok {
		var obj Obj
		decoder := xml.NewDecoder(strings.NewReader(f.Detail))
		err := decoder.Decode(&obj)
		if err != nil {
			return nil, fmt.Errorf("failed to decode from xml to fault: %w", err)
		}
		return &obj.Fault, nil
	}
	return nil, fmt.Errorf("error is not of type esx.Fault")
}
