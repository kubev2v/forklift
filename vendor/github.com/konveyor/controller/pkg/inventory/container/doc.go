// Container
//   |__Reconciler
//   |__Reconciler
//   |__Reconciler
//
// The container is a collection of data model reconcilers.
// Each reconciler is responsible for ensuring that changes made
// to the external data source are reflected in the DB.  The
// goal is for the data model to be eventually consistent.
package container

//
// Build a new container.
func New() *Container {
	return &Container{
		content: map[Key]Reconciler{},
	}
}
