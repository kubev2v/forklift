package tls

import (
	"net/url"
)

// InsecureFromLibvirtURL reports whether V2V_libvirtURL has no_verify=1.
func InsecureFromLibvirtURL(libvirtURL string) bool {
	u, err := url.Parse(libvirtURL)
	if err != nil {
		return false
	}
	return InsecureFromQuery(u.RawQuery)
}

// InsecureFromQuery reads no_verify from a libvirt URL query string.
// Insecure mode is enabled only when no_verify=1 exactly.
func InsecureFromQuery(rawQuery string) bool {
	if rawQuery == "" {
		return false
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return false
	}
	return values.Get("no_verify") == "1"
}

// ParseLibvirtTLS reads TLS mode from a libvirt/vpx URL query string.
// The cacert= parameter is Forklift boilerplate and is not used for TLS mode selection.
func ParseLibvirtTLS(libvirtURL string) (insecure bool, _ string) {
	u, err := url.Parse(libvirtURL)
	if err != nil {
		return false, ""
	}
	insecure = InsecureFromQuery(u.RawQuery)
	return insecure, ""
}

// ParseQueryTLS reads no_verify from a libvirt URL query string.
// Deprecated: use InsecureFromQuery. The returned caBundle is always empty.
func ParseQueryTLS(rawQuery string) (insecure bool, caBundle string) {
	return InsecureFromQuery(rawQuery), ""
}
