/*
//
// Create a remote (cluster).
remote := &watch.Remote{
    RestCfg: restCfg,
}

//
// Create a remote (cluster) with relay to a controller.
remote := &watch.Remote{
    RestCfg: restCfg,
    Relay: watch.Relay{
        Controller: controller,
        Object: object,
    }
}

//
// Add watch(s) and start the remote.
remote.Start(
    watch.Watch{
        Object: &v1.Pod{},
        Predicates: []predicate{
            &predicate{},
        },
        Object: &v1.Secret{},
        Predicates: []predicate{
            &predicate{},
        },
    })

//
// Shutdown the remote.
remote.Shutdown()

//
// Add individual watch.
remote.Watch(
    &source.Kind{
        Type: &v1.Secret{},
    },
    &MyPredicate{})

//
// Register your remote.
watch.Map.Add(myObject, remote)

//
// Find a registered remote.
remote, found := watch.Map.Find(myObject)

//
// Unregister a registered remote.
remote, found := watch.Map.Delete(myObject)
*/
package watch
