package main

import (
	"strings"
	"testing"
)

func TestGenName(t *testing.T) {
	cases := []struct {
		diskNum  int
		expected string
	}{
		{1, "a"},
		{26, "z"},
		{27, "aa"},
		{28, "ab"},
		{52, "az"},
		{53, "ba"},
		{55, "bc"},
		{702, "zz"},
		{754, "abz"},
	}

	for _, c := range cases {
		got := genName(c.diskNum)
		if got != c.expected {
			t.Errorf("genName(%d) = %s; want %s", c.diskNum, got, c.expected)
		}
	}
}

func TestStaticIPs(t *testing.T) {
	t.Setenv("V2V_source", VSPHERE)
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
		command := strings.Join(buildCommand(), " ")
		for _, outputArg := range c.outputArgs {
			if !strings.Contains(command, outputArg) {
				t.Errorf("The command is: %s. Excpected to contain '%s'", command, outputArg)
			}
		}
	}
}
