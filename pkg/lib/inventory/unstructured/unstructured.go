package unstructured

import (
	k8sunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Unstructured wraps Kubernetes' Unstructured type to provide simpler accessor methods
// that work with simple path strings (from schema mappings) rather than variadic field arguments.
//
// This makes the code much cleaner:
//
//	vm.GetString("id")  vs  unstructured.NestedString(vm.Object, "id")
//
// It delegates to Kubernetes' standard unstructured helpers for robust field access.
type Unstructured struct {
	k8sunstructured.Unstructured
}

// GetString retrieves a string value from the nested map at the given path.
// The path is typically a simple field name like "id", "name", etc.
// Returns the string value and a boolean indicating if the field was found.
func (u *Unstructured) GetString(path string) (string, bool) {
	val, found, _ := k8sunstructured.NestedString(u.Object, path)
	return val, found
}

// GetInt retrieves an int value from the nested map at the given path.
// Returns the int value and a boolean indicating if the field was found.
func (u *Unstructured) GetInt(path string) (int, bool) {
	val, found, _ := k8sunstructured.NestedInt64(u.Object, path)
	return int(val), found
}

// GetInt64 retrieves an int64 value from the nested map at the given path.
// Returns the int64 value and a boolean indicating if the field was found.
func (u *Unstructured) GetInt64(path string) (int64, bool) {
	val, found, _ := k8sunstructured.NestedInt64(u.Object, path)
	return val, found
}

// GetSlice retrieves a slice from the nested map at the given path.
// Returns the slice value and a boolean indicating if the field was found.
func (u *Unstructured) GetSlice(path string) ([]interface{}, bool) {
	val, found, _ := k8sunstructured.NestedSlice(u.Object, path)
	return val, found
}

// GetBool retrieves a bool value from the nested map at the given path.
// Returns the bool value and a boolean indicating if the field was found.
func (u *Unstructured) GetBool(path string) (bool, bool) {
	val, found, _ := k8sunstructured.NestedBool(u.Object, path)
	return val, found
}
