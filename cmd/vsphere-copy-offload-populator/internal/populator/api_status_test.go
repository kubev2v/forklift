package populator

import (
	"context"
	"testing"
	"time"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/vmware"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func TestAPITaskStatus(t *testing.T) {
	model := simulator.ESX()
	if err := model.Create(); err != nil {
		t.Fatal(err)
	}
	defer model.Remove()
	server := model.Service.NewServer()
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	host, err := vmware.NewHostClient(ctx, server.URL.String(), "user", "pass", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close(ctx)
	executor := NewAPITaskExecutor(host, "disk-test")

	for _, tt := range []struct {
		name     string
		info     types.TaskInfo
		exitCode string
		progress string
		stderr   string
	}{
		{name: "queued", info: types.TaskInfo{State: types.TaskInfoStateQueued}, progress: "0%"},
		{name: "running", info: types.TaskInfo{State: types.TaskInfoStateRunning, Progress: 42}, progress: "42%"},
		{name: "completed", info: types.TaskInfo{State: types.TaskInfoStateSuccess}, exitCode: "0", progress: "100%"},
		{name: "failed", info: types.TaskInfo{State: types.TaskInfoStateError, Error: &types.LocalizedMethodFault{LocalizedMessage: "destination unavailable"}}, exitCode: "1", progress: "0%", stderr: "destination unavailable"},
		{name: "failed without fault", info: types.TaskInfo{State: types.TaskInfoStateError}, exitCode: "1", progress: "0%", stderr: "CopyVirtualDisk task failed without a reported fault"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			entity := host.Host.Reference()
			tt.info.Entity = &entity
			task := &simulator.Task{Task: mo.Task{Info: tt.info}}
			ref := model.Map().Put(task).Reference()
			status, err := executor.GetTaskStatus(ctx, nil, "", ref.Value)
			if err != nil {
				t.Fatal(err)
			}
			if status.TaskId != ref.Value || status.ExitCode != tt.exitCode || status.LastLine != tt.progress || status.Stderr != tt.stderr {
				t.Fatalf("unexpected task status: %+v", status)
			}
		})
	}
	if _, err := executor.GetTaskStatus(ctx, nil, "", "missing-task"); err == nil {
		t.Fatal("missing task must return an error")
	}
}
