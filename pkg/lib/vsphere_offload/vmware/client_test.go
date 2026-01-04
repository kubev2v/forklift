package vmware

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/simulator"
)

func TestNewClientWithSimulator(t *testing.T) {
	model := simulator.VPX()
	defer model.Remove()

	err := model.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := model.Service.NewServer()
	defer s.Close()

	_, err = NewClient(s.URL.String(), "user", "pass")
	if err != nil {
		t.Errorf("NewClient() error = %v, wantErr %v", err, false)
	}
}

func TestVSphereClient_GetEsxByVm(t *testing.T) {
	model := simulator.VPX()
	defer model.Remove()

	err := model.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := model.Service.NewServer()
	defer s.Close()

	client, err := NewClient(s.URL.String(), "user", "pass")
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetEsxByVm(context.TODO(), "vm-1")
	if err == nil {
		t.Errorf("GetEsxByVm() error = %v, wantErr %v", err, true)
	}
}

func TestVSphereClient_GetDatastore(t *testing.T) {
	model := simulator.VPX()
	defer model.Remove()

	err := model.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := model.Service.NewServer()
	defer s.Close()

	client, err := NewClient(s.URL.String(), "user", "pass")
	if err != nil {
		t.Fatal(err)
	}

	finder := find.NewFinder(client.(*VSphereClient).Client.Client, false)
	dc, err := finder.DefaultDatacenter(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetDatastore(context.TODO(), dc, "LocalDS_0")
	if err != nil {
		t.Errorf("GetDatastore() error = %v, wantErr %v", err, false)
	}
}

func TestVSphereClient_RunEsxCommand(t *testing.T) {
	t.Skip("Skipping test that requires esxcli executor on simulator")
	model := simulator.VPX()
	defer model.Remove()

	err := model.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := model.Service.NewServer()
	defer s.Close()

	client, err := NewClient(s.URL.String(), "user", "pass")
	if err != nil {
		t.Fatal(err)
	}

	finder := find.NewFinder(client.(*VSphereClient).Client.Client, false)
	dc, err := finder.DefaultDatacenter(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	finder.SetDatacenter(dc)

	host, err := finder.HostSystem(context.TODO(), "host-21")
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.RunEsxCommand(context.TODO(), host, []string{"echo", "hello"})
	if err != nil {
		t.Errorf("RunEsxCommand() error = %v, wantErr %v", err, false)
	}
}
