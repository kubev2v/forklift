package ocp

type Resource struct {
	UID       string `json:"uid"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	SelfLink  string `json:"selfLink"`
}
