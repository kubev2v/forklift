package main

import "testing"

func TestEntityDownloadToV3FileURL(t *testing.T) {
	got, ok := entityDownloadToV3FileURL("https://pe.example:9440/entity_download/abc-uuid")
	if !ok {
		t.Fatal("expected rewrite")
	}
	want := "https://pe.example:9440/api/nutanix/v3/images/abc-uuid/file"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if _, ok := entityDownloadToV3FileURL("https://pe.example:9440/api/nutanix/v3/images/abc/file"); ok {
		t.Fatal("v3 /file must not rewrite")
	}
}
