package utils

import (
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
