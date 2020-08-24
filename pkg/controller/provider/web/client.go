package web

//
// Client resource.
type ClientResource = interface{}

//
// Client.
type Client interface {
	// Get a resource.
	Get(interface{}, string) (int, error)
	// List resources.
	List(interface{}) (int, error)
}
