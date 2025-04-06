package templateutil

import (
	"math"
)

// Add safely adds a series of numbers.
func Add(values ...interface{}) int64 {
	var sum int64
	for _, v := range values {
		if i, ok := toInt64(v); ok {
			sum += i
		}
	}
	return sum
}

// Add1 safely increments a number by 1.
func Add1(value interface{}) int64 {
	if i, ok := toInt64(value); ok {
		return i + 1
	}
	return 1
}

// Sub safely subtracts the second number from the first.
func Sub(a, b interface{}) int64 {
	aVal, aOk := toInt64(a)
	bVal, bOk := toInt64(b)
	if !aOk || !bOk {
		return 0
	}
	return aVal - bVal
}

// Div safely divides the first number by the second.
func Div(a, b interface{}) int64 {
	aVal, aOk := toInt64(a)
	bVal, bOk := toInt64(b)
	if !aOk || !bOk || bVal == 0 {
		return 0
	}
	return aVal / bVal
}

// Mod safely computes the modulus of the first number divided by the second.
func Mod(a, b interface{}) int64 {
	aVal, aOk := toInt64(a)
	bVal, bOk := toInt64(b)
	if !aOk || !bOk || bVal == 0 {
		return 0
	}
	return aVal % bVal
}

// Mul safely multiplies a series of numbers.
func Mul(values ...interface{}) int64 {
	if len(values) == 0 {
		return 0
	}

	product, ok := toInt64(values[0])
	if !ok {
		return 0
	}

	for i := 1; i < len(values); i++ {
		if i, ok := toInt64(values[i]); ok {
			product *= i
		} else {
			return 0
		}
	}
	return product
}

// Max safely returns the largest of a series of integers.
func Max(values ...interface{}) int64 {
	if len(values) == 0 {
		return 0
	}

	maxVal, ok := toInt64(values[0])
	if !ok {
		return 0
	}

	for i := 1; i < len(values); i++ {
		if i, ok := toInt64(values[i]); ok && i > maxVal {
			maxVal = i
		}
	}
	return maxVal
}

// Min safely returns the smallest of a series of integers.
func Min(values ...interface{}) int64 {
	if len(values) == 0 {
		return 0
	}

	minVal, ok := toInt64(values[0])
	if !ok {
		return 0
	}

	for i := 1; i < len(values); i++ {
		if i, ok := toInt64(values[i]); ok && i < minVal {
			minVal = i
		}
	}
	return minVal
}

// Floor returns the greatest float value less than or equal to input value.
func Floor(value interface{}) float64 {
	if f, ok := toFloat64(value); ok {
		return math.Floor(f)
	}
	return 0.0
}

// Ceil returns the smallest float value greater than or equal to input value.
func Ceil(value interface{}) float64 {
	if f, ok := toFloat64(value); ok {
		return math.Ceil(f)
	}
	return 0.0
}

// Round returns a float value with the remainder rounded to the given digits after decimal point.
func Round(value, digits interface{}) float64 {
	f, fOk := toFloat64(value)
	d, dOk := toInt64(digits)
	if !fOk || !dOk {
		return 0.0
	}

	precision := math.Pow10(int(d))
	return math.Round(f*precision) / precision
}

// Helper function to convert an interface{} to int64
func toInt64(v interface{}) (int64, bool) {
	if v == nil {
		return 0, false
	}

	switch i := v.(type) {
	case int:
		return int64(i), true
	case int8:
		return int64(i), true
	case int16:
		return int64(i), true
	case int32:
		return int64(i), true
	case int64:
		return i, true
	case uint:
		return int64(i), true
	case uint8:
		return int64(i), true
	case uint16:
		return int64(i), true
	case uint32:
		return int64(i), true
	case uint64:
		return int64(i), true
	case float32:
		return int64(i), true
	case float64:
		return int64(i), true
	}

	return 0, false
}

// Helper function to convert an interface{} to float64
func toFloat64(v interface{}) (float64, bool) {
	if v == nil {
		return 0, false
	}

	switch i := v.(type) {
	case int:
		return float64(i), true
	case int8:
		return float64(i), true
	case int16:
		return float64(i), true
	case int32:
		return float64(i), true
	case int64:
		return float64(i), true
	case uint:
		return float64(i), true
	case uint8:
		return float64(i), true
	case uint16:
		return float64(i), true
	case uint32:
		return float64(i), true
	case uint64:
		return float64(i), true
	case float32:
		return float64(i), true
	case float64:
		return i, true
	}

	return 0, false
}
