package customize

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customize/advancednet"
)

func TestRenderTemplate(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "test.ps1.tmpl")
	outPath := filepath.Join(dir, "test.ps1")

	if err := os.WriteFile(tmplPath, []byte("Hello {{.Name}}, value={{.Value}}"), 0644); err != nil {
		t.Fatal(err)
	}

	data := struct {
		Name  string
		Value int
	}{"world", 42}

	if err := renderTemplate(tmplPath, outPath, data); err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	expected := "Hello world, value=42"
	if string(got) != expected {
		t.Errorf("expected %q, got %q", expected, string(got))
	}
}

func TestRenderTemplate_MissingTemplate(t *testing.T) {
	dir := t.TempDir()
	err := renderTemplate("/nonexistent/template.tmpl", filepath.Join(dir, "out.ps1"), nil)
	if err == nil {
		t.Error("expected error for missing template")
	}
}

func TestInjectAdvancedNetworkSettings_NoFile(t *testing.T) {
	dir := t.TempDir()
	c := &Customize{
		appConfig: &config.AppConfig{Workdir: dir},
	}
	err := c.injectAdvancedNetworkSettingsTemplate(dir)
	if !errors.Is(err, ErrNoAdvancedNetSettings) {
		t.Errorf("expected ErrNoAdvancedNetSettings, got %v", err)
	}
}

func TestInjectAdvancedNetworkSettings_AllDefaults(t *testing.T) {
	dir := t.TempDir()
	settings := &advancednet.AdvancedNetSettings{
		LanmanServerStart: advancednet.LanmanServerStartAutomatic,
	}
	if err := advancednet.WriteSettingsFile(settings, dir); err != nil {
		t.Fatal(err)
	}
	c := &Customize{
		appConfig: &config.AppConfig{Workdir: dir},
	}
	err := c.injectAdvancedNetworkSettingsTemplate(dir)
	if !errors.Is(err, ErrNoAdvancedNetSettings) {
		t.Errorf("expected ErrNoAdvancedNetSettings for default settings, got %v", err)
	}
}

func TestInjectAdvancedNetworkSettings_RendersJSONScript(t *testing.T) {
	dir := t.TempDir()

	settings := &advancednet.AdvancedNetSettings{
		Interfaces: []advancednet.InterfaceSettings{
			{
				MAC:                 "00:50:56:BE:56:A1",
				InterfaceMetric:     25,
				RegistrationEnabled: 0,
				NetbiosOptions:      2,
			},
		},
		LanmanServerStart:          4,
		FilePrinterSharingDisabled: []advancednet.AdapterRef{{GUID: "{TEST}", MAC: "00:50:56:BE:56:A1"}},
	}
	if err := advancednet.WriteSettingsFile(settings, dir); err != nil {
		t.Fatal(err)
	}

	tmplPath := filepath.Join(dir, "9998-advanced-network-settings.ps1.tmpl")
	tmplContent := "$settingsJson = '{{.SettingsJSON}}'\n"
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0644); err != nil {
		t.Fatal(err)
	}

	c := &Customize{
		appConfig: &config.AppConfig{Workdir: dir},
	}
	if err := c.injectAdvancedNetworkSettingsTemplate(dir); err != nil {
		t.Fatalf("injectAdvancedNetworkSettingsTemplate: %v", err)
	}

	outPath := filepath.Join(dir, "9998-advanced-network-settings.ps1")
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	rendered := string(got)

	prefix := "$settingsJson = '"
	suffix := "'"
	start := strings.Index(rendered, prefix)
	if start == -1 {
		t.Fatalf("rendered script missing settingsJson assignment:\n%s", rendered)
	}
	start += len(prefix)
	end := strings.Index(rendered[start:], suffix)
	if end == -1 {
		t.Fatalf("rendered script missing closing quote:\n%s", rendered)
	}
	jsonValue := rendered[start : start+end]

	var roundTrip advancednet.AdvancedNetSettings
	if err := json.Unmarshal([]byte(jsonValue), &roundTrip); err != nil {
		t.Fatalf("JSON unmarshal failed: %v\nraw: %s", err, jsonValue)
	}
	if len(roundTrip.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(roundTrip.Interfaces))
	}
	if roundTrip.Interfaces[0].InterfaceMetric != 25 {
		t.Errorf("InterfaceMetric: expected 25, got %d", roundTrip.Interfaces[0].InterfaceMetric)
	}
	if roundTrip.LanmanServerStart != 4 {
		t.Errorf("LanmanServerStart: expected 4, got %d", roundTrip.LanmanServerStart)
	}
	if len(roundTrip.FilePrinterSharingDisabled) != 1 {
		t.Errorf("FilePrinterSharingDisabled: expected 1, got %d", len(roundTrip.FilePrinterSharingDisabled))
	}
}

func TestInjectAdvancedNetworkSettings_RealFailurePropagates(t *testing.T) {
	dir := t.TempDir()

	settings := &advancednet.AdvancedNetSettings{
		LanmanServerStart: 4,
	}
	if err := advancednet.WriteSettingsFile(settings, dir); err != nil {
		t.Fatal(err)
	}

	c := &Customize{
		appConfig: &config.AppConfig{Workdir: dir},
	}
	// No template file exists so renderTemplate should fail with a real error
	err := c.injectAdvancedNetworkSettingsTemplate(dir)
	if err == nil {
		t.Fatal("expected error when template file is missing")
	}
	if errors.Is(err, ErrNoAdvancedNetSettings) {
		t.Error("missing template should not return ErrNoAdvancedNetSettings")
	}
}
