package main

import (
	"strings"
	"testing"

	"github.com/konveyor/forklift-controller/pkg/virt-v2v/global"
)

func TestStaticIPs(t *testing.T) {
	t.Setenv("V2V_source", global.VSPHERE)
	t.Setenv("V2V_libvirtURL", "http://fake.com")
	t.Setenv("V2V_secretKey", "fake")
	t.Setenv("V2V_vmName", "test")

	cases := []struct {
		inputConfig string
		outputArgs  []string
	}{
		{"", []string{""}},
		{"00:50:56:83:25:47:ip:172.29.3.193", []string{"--mac 00:50:56:83:25:47:ip:172.29.3.193"}},
		{"00:50:56:83:25:47:ip:172.29.3.193_00:50:56:83:25:47:ip:fe80::5da:b7a5:e0a2:a097", []string{"--mac 00:50:56:83:25:47:ip:172.29.3.193", "--mac 00:50:56:83:25:47:ip:fe80::5da:b7a5:e0a2:a097"}},
	}

	for _, c := range cases {
		if c.inputConfig != "" {
			t.Setenv("V2V_staticIPs", c.inputConfig)
		}
		args, err := virtV2vBuildCommand()
		if err != nil {
			t.Error("Failed to build command", err)
		}
		command := strings.Join(args, " ")
		for _, outputArg := range c.outputArgs {
			if !strings.Contains(command, outputArg) {
				t.Errorf("The command is: %s. Excpected to contain '%s'", command, outputArg)
			}
		}
	}
}
