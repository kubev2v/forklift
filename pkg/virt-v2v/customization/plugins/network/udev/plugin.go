// network/udev — Linux NIC renaming via persistent udev rules.
//
// Applicable: Linux + StaticIPs set + probe found interfaces.
// Output:     FileAction{Write} — /etc/udev/rules.d/70-persistent-net.rules
//
// Parses the StaticIPs string (MAC:ip:IP,..._MAC:ip:IP,...) into MAC/IP
// pairs, looks up each IP in the probe-extracted GuestInfo.Interfaces, and
// writes udev rules that bind each MAC to its original interface name.
package udev

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
)

type Plugin struct{}

func (p *Plugin) Name() string { return "network/udev" }

func (p *Plugin) Applicable(ctx *api.Context) bool {
	if ctx == nil || ctx.Guest == nil || ctx.Config == nil {
		return false
	}
	return ctx.Guest.OS.IsLinux() && ctx.Config.StaticIPs != "" && len(ctx.Guest.Interfaces) > 0
}

func (p *Plugin) Apply(ctx *api.Context) (*api.Actions, error) {
	if ctx == nil || ctx.Guest == nil || ctx.Config == nil {
		return nil, fmt.Errorf("udev plugin: nil context, guest, or config")
	}
	pairs := parseStaticIPs(ctx.Config.StaticIPs)
	var rules []string
	for _, pair := range pairs {
		if _, err := net.ParseMAC(pair.MAC); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping invalid MAC %q in StaticIPs: %v\n", pair.MAC, err)
			continue
		}
		if net.ParseIP(pair.IP) == nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping invalid IP %q in StaticIPs\n", pair.IP)
			continue
		}
		ifaceName := ctx.Guest.InterfaceForIP(pair.IP)
		if ifaceName == "" {
			continue
		}
		rules = append(rules, fmt.Sprintf(
			`SUBSYSTEM=="net",ACTION=="add",ATTR{address}=="%s",NAME="%s"`,
			strings.ToLower(pair.MAC), ifaceName))
	}
	if len(rules) == 0 {
		return &api.Actions{}, nil
	}
	return &api.Actions{
		Files: []api.FileAction{{
			Type:        api.ActionWrite,
			GuestPath:   "/etc/udev/rules.d/70-persistent-net.rules",
			Content:     []byte(strings.Join(rules, "\n") + "\n"),
			Permissions: "0644",
		}},
	}, nil
}

type macIPPair struct {
	MAC string
	IP  string
}

// parseStaticIPs splits a "MAC:ip:IP,..._MAC:ip:IP,..." string into MAC/IP pairs.
func parseStaticIPs(staticIPs string) []macIPPair {
	var pairs []macIPPair
	segments := strings.Split(staticIPs, "_")
	for _, seg := range segments {
		parts := strings.SplitN(seg, ":ip:", 2)
		if len(parts) != 2 {
			continue
		}
		mac := parts[0]
		ipPart := parts[1]
		ip := strings.SplitN(ipPart, ",", 2)[0]
		if mac != "" && ip != "" {
			pairs = append(pairs, macIPPair{MAC: mac, IP: ip})
		}
	}
	return pairs
}
