// Container
//   |__Reconciler
//   |__Reconciler
//   |__Reconciler
//
package container

//
// Build a new container.
func New() *Container {
	return &Container{
		content: map[Key]Reconciler{},
	}
}
