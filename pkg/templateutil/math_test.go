package templateutil

import (
	"testing"
)

func TestAdd(t *testing.T) {
	tests := []struct {
		name     string
		values   []interface{}
		expected int64
	}{
		{"Empty args", []interface{}{}, 0},
		{"Single value", []interface{}{5}, 5},
		{"Multiple values", []interface{}{1, 2, 3, 4}, 10},
		{"With nil", []interface{}{1, nil, 3}, 4},
		{"Mixed types", []interface{}{1, 2.5, int32(3), uint(4)}, 10},
		{"Non-numeric", []interface{}{"string", true}, 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := Add(test.values...)
			if result != test.expected {
				t.Errorf("Add(%v) = %v, expected %v", test.values, result, test.expected)
			}
		})
	}
}

func TestAdd1(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected int64
	}{
		{"Normal integer", 5, 6},
		{"Zero", 0, 1},
		{"Negative", -5, -4},
		{"Float", 5.7, 6},
		{"Nil", nil, 1},
		{"Non-numeric", "string", 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := Add1(test.value)
			if result != test.expected {
				t.Errorf("Add1(%v) = %v, expected %v", test.value, result, test.expected)
			}
		})
	}
}

func TestSub(t *testing.T) {
	tests := []struct {
		name     string
		a        interface{}
		b        interface{}
		expected int64
	}{
		{"Normal case", 10, 3, 7},
		{"Zero result", 5, 5, 0},
		{"Negative result", 5, 10, -5},
		{"Mixed types", 10.5, int32(3), 7},
		{"First nil", nil, 5, 0},
		{"Second nil", 5, nil, 0},
		{"Both nil", nil, nil, 0},
		{"Non-numeric", "string", 5, 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := Sub(test.a, test.b)
			if result != test.expected {
				t.Errorf("Sub(%v, %v) = %v, expected %v", test.a, test.b, result, test.expected)
			}
		})
	}
}

func TestDiv(t *testing.T) {
	tests := []struct {
		name     string
		a        interface{}
		b        interface{}
		expected int64
	}{
		{"Normal case", 10, 2, 5},
		{"Integer division", 10, 3, 3},
		{"Zero dividend", 0, 5, 0},
		{"Zero divisor", 5, 0, 0},
		{"Negative result", 10, -2, -5},
		{"Mixed types", 10.9, int32(2), 5},
		{"First nil", nil, 5, 0},
		{"Second nil", 5, nil, 0},
		{"Both nil", nil, nil, 0},
		{"Non-numeric", "string", 5, 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := Div(test.a, test.b)
			if result != test.expected {
				t.Errorf("Div(%v, %v) = %v, expected %v", test.a, test.b, result, test.expected)
			}
		})
	}
}

func TestRound(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		digits   interface{}
		expected float64
	}{
		{"Round to integer", 3.75, 0, 4.0},
		{"Round to one decimal", 3.75, 1, 3.8},
		{"Round to two decimals", 3.75159, 2, 3.75},
		{"Negative digits", 3.75, -1, 0.0},
		{"Nil value", nil, 2, 0.0},
		{"Nil digits", 3.75, nil, 0.0},
		{"Non-numeric", "string", 2, 0.0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := Round(test.value, test.digits)
			if result != test.expected {
				t.Errorf("Round(%v, %v) = %v, expected %v",
					test.value, test.digits, result, test.expected)
			}
		})
	}
}
