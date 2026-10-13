package vmware

import (
	"context"
	"crypto/tls"
	"encoding/pem"
	"testing"
	"time"

	"github.com/vmware/govmomi/simulator"
)

func TestHostClientTLS(t *testing.T) {
	model := simulator.ESX()
	if err := model.Create(); err != nil {
		t.Fatal(err)
	}
	defer model.Remove()
	model.Service.TLS = new(tls.Config)
	server := model.Service.NewServer()
	defer server.Close()
	caCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})

	for _, tt := range []struct {
		name     string
		insecure bool
		caCert   []byte
		wantErr  bool
	}{
		{name: "untrusted certificate rejected", wantErr: true},
		{name: "explicit insecure setting", insecure: true},
		{name: "Host secret CA trusted", caCert: caCert},
		{name: "invalid CA rejected", caCert: []byte("invalid"), wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			client, err := NewHostClient(ctx, server.URL.String(), "user", "pass", tt.insecure, tt.caCert)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewHostClient error = %v, want error: %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			defer client.Close(ctx)
			if client.Host == nil || client.Datacenter == nil {
				t.Fatal("host SDK inventory was not resolved")
			}
			if _, err := client.ResolveDeviceName(ctx, "naa.missing"); err == nil {
				t.Fatal("missing destination LUN was accepted")
			}
		})
	}
}
