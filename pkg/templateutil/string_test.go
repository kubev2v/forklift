package templateutil

import (
	"testing"
)

func TestToLower(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"Normal string", "HELLO", "hello"},
		{"Mixed case", "HeLLo", "hello"},
		{"Empty string", "", ""},
		{"Nil input", nil, ""},
		{"Non-string input", 123, ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ToLower(test.input)
			if result != test.expected {
				t.Errorf("ToLower(%v) = %v, expected %v", test.input, result, test.expected)
			}
		})
	}
}

func TestToUpper(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"Normal string", "hello", "HELLO"},
		{"Mixed case", "HeLLo", "HELLO"},
		{"Empty string", "", ""},
		{"Nil input", nil, ""},
		{"Non-string input", 123, ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ToUpper(test.input)
			if result != test.expected {
				t.Errorf("ToUpper(%v) = %v, expected %v", test.input, result, test.expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        interface{}
		substr   interface{}
		expected bool
	}{
		{"Normal case", "hello world", "world", true},
		{"Not contained", "hello world", "universe", false},
		{"Empty string", "", "", true},
		{"Nil string", nil, "test", false},
		{"Nil substring", "test", nil, false},
		{"Both nil", nil, nil, false},
		{"Non-string inputs", 123, 456, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := Contains(test.s, test.substr)
			if result != test.expected {
				t.Errorf("Contains(%v, %v) = %v, expected %v", test.s, test.substr, result, test.expected)
			}
		})
	}
}

func TestReplace(t *testing.T) {
	tests := []struct {
		name     string
		s        interface{}
		old      interface{}
		new      interface{}
		n        int
		expected string
	}{
		{"Replace all", "hello hello", "l", "x", -1, "hexxo hexxo"},
		{"Replace first two", "hello hello", "l", "x", 2, "hexxo hello"},
		{"Empty string", "", "l", "x", -1, ""},
		{"Nil string", nil, "l", "x", -1, ""},
		{"Nil old", "hello", nil, "x", -1, ""},
		{"Nil new", "hello", "l", nil, -1, ""},
		{"Non-string inputs", 123, 456, 789, -1, ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := Replace(test.s, test.old, test.new, test.n)
			if result != test.expected {
				t.Errorf("Replace(%v, %v, %v, %v) = %v, expected %v",
					test.s, test.old, test.new, test.n, result, test.expected)
			}
		})
	}
}

func TestTrim(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"Spaces both ends", "  hello  ", "hello"},
		{"No spaces", "hello", "hello"},
		{"Only spaces", "   ", ""},
		{"Empty string", "", ""},
		{"Nil input", nil, ""},
		{"Non-string input", 123, ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := Trim(test.input)
			if result != test.expected {
				t.Errorf("Trim(%v) = %v, expected %v", test.input, result, test.expected)
			}
		})
	}
}

// Additional tests for string functions can be implemented similarly
func TestInitials(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"Normal case", "John Doe", "JD"},
		{"Multiple words", "The Quick Brown Fox", "TQBF"},
		{"Single word", "Hello", "H"},
		{"Empty string", "", ""},
		{"Nil input", nil, ""},
		{"Non-string input", 123, ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := Initials(test.input)
			if result != test.expected {
				t.Errorf("Initials(%v) = %v, expected %v", test.input, result, test.expected)
			}
		})
	}
}
