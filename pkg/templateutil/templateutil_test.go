package templateutil

import (
	"testing"
	"text/template"
)

func TestAddStringFuncs(t *testing.T) {
	// Test adding string functions to nil FuncMap
	funcMap := AddStringFuncs(nil)
	if funcMap == nil {
		t.Errorf("AddStringFuncs(nil) returned nil FuncMap")
	}

	// Test that all string functions are added
	expectedFuncs := []string{
		"lower", "upper", "contains", "replace", "trim",
		"trimAll", "trimSuffix", "trimPrefix", "title",
		"untitle", "repeat", "substr", "nospace", "trunc",
		"initials", "hasPrefix", "hasSuffix", "mustRegexReplaceAll",
	}

	for _, funcName := range expectedFuncs {
		if _, exists := funcMap[funcName]; !exists {
			t.Errorf("Function %s not added to FuncMap", funcName)
		}
	}

	// Test adding to existing FuncMap
	existingMap := template.FuncMap{
		"customFunc": func() string { return "custom" },
	}

	updatedMap := AddStringFuncs(existingMap)
	if _, exists := updatedMap["customFunc"]; !exists {
		t.Errorf("Existing function customFunc was removed from FuncMap")
	}
}

func TestAddMathFuncs(t *testing.T) {
	// Test adding math functions to nil FuncMap
	funcMap := AddMathFuncs(nil)
	if funcMap == nil {
		t.Errorf("AddMathFuncs(nil) returned nil FuncMap")
	}

	// Test that all math functions are added
	expectedFuncs := []string{
		"add", "add1", "sub", "div", "mod", "mul",
		"max", "min", "floor", "ceil", "round",
	}

	for _, funcName := range expectedFuncs {
		if _, exists := funcMap[funcName]; !exists {
			t.Errorf("Function %s not added to FuncMap", funcName)
		}
	}

	// Test adding to existing FuncMap
	existingMap := template.FuncMap{
		"customFunc": func() string { return "custom" },
	}

	updatedMap := AddMathFuncs(existingMap)
	if _, exists := updatedMap["customFunc"]; !exists {
		t.Errorf("Existing function customFunc was removed from FuncMap")
	}
}

func TestAddTemplateFuncs(t *testing.T) {
	// Test that AddTemplateFuncs adds both string and math functions
	funcMap := AddTemplateFuncs(nil)

	// Check for string function
	if _, exists := funcMap["upper"]; !exists {
		t.Errorf("String function 'upper' not added by AddTemplateFuncs")
	}

	// Check for math function
	if _, exists := funcMap["add"]; !exists {
		t.Errorf("Math function 'add' not added by AddTemplateFuncs")
	}
}

func TestExecuteTemplate(t *testing.T) {
	tests := []struct {
		name         string
		templateText string
		data         interface{}
		expected     string
		expectError  bool
	}{
		{
			name:         "Simple template",
			templateText: "Hello, {{.Name}}!",
			data:         map[string]interface{}{"Name": "World"},
			expected:     "Hello, World!",
			expectError:  false,
		},
		{
			name:         "Using upper function",
			templateText: "Hello, {{upper .Name}}!",
			data:         map[string]interface{}{"Name": "World"},
			expected:     "Hello, WORLD!",
			expectError:  false,
		},
		{
			name:         "Using math function",
			templateText: "Result: {{add 1 2 3}}",
			data:         nil,
			expected:     "Result: 6",
			expectError:  false,
		},
		{
			name:         "Multiple functions",
			templateText: "{{upper (trimPrefix \"Hello, \" .Greeting)}}",
			data:         map[string]interface{}{"Greeting": "Hello, World!"},
			expected:     "WORLD!",
			expectError:  false,
		},
		{
			name:         "Invalid syntax",
			templateText: "Hello, {{.Name}",
			data:         map[string]interface{}{"Name": "World"},
			expected:     "",
			expectError:  true,
		},
		{
			name:         "Using mustRegexReplaceAll function",
			templateText: `{{mustRegexReplaceAll "a(x*)b" "-ab-axxb-" "${1}W"}}`,
			data:         nil,
			expected:     "-W-xxW-",
			expectError:  false,
		},
		{
			name:         "Complex regex replacement",
			templateText: `{{mustRegexReplaceAll "([a-z]+)([0-9]+)" .Input "$2-$1"}}`,
			data:         map[string]interface{}{"Input": "test123word456"},
			expected:     "123-test456-word",
			expectError:  false,
		},
		{
			name:         "Drive letter extraction with formatting",
			templateText: `disk-{{ mustRegexReplaceAll "([A-Za-z]+):.*" .Input "$1" | lower }}`,
			data:         map[string]interface{}{"Input": "C:\\"},
			expected:     "disk-c",
			expectError:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := ExecuteTemplate(test.templateText, test.data)

			if test.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !test.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !test.expectError && result != test.expected {
				t.Errorf("ExecuteTemplate() = %v, expected %v", result, test.expected)
			}
		})
	}
}
