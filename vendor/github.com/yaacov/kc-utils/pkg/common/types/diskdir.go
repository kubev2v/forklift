package types

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

// diskImageNameRE matches standalone copy/prepare images: disk0.img, disk1.img, …
var diskImageNameRE = regexp.MustCompile(`^disk(\d+)\.img$`)

// ImageFileName returns the standalone raw image name for index N (disk0.img).
func ImageFileName(index int) string {
	return fmt.Sprintf("disk%d.img", index)
}

// ExpandDiskDir returns diskN.img files under dir as raw DiskSpecs, sorted by N.
func ExpandDiskDir(dir string) ([]DiskSpec, error) {
	if dir == "" {
		return nil, fmt.Errorf("disk_dir is empty")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("disk_dir %s: %w", dir, err)
	}

	type found struct {
		path  string
		index int
	}
	var files []found
	for _, entry := range entries {
		m := diskImageNameRE.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}
		if !entry.Type().IsRegular() {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		files = append(files, found{
			path:  filepath.Join(dir, entry.Name()),
			index: n,
		})
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no diskN.img files in %s", dir)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].index < files[j].index
	})

	out := make([]DiskSpec, len(files))
	for i, f := range files {
		out[i] = DiskSpec{Path: f.path, Format: "raw"}
	}
	return out, nil
}
