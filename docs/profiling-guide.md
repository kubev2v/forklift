# Profiling of Forklift
This is profiling doc for the forklift which contains few examples. It is not full documentaiton on how to use the
pprof, for that please visit the [pprof](https://pkg.go.dev/net/http/pprof).

### Forward the pods profiling port
The pprof is running within the inventory and controller container. The port is not exposed to so you need to forward
the port to your local machine with the following command:

`oc port-forward -n konveyor-forklift pods/forklift-controller-5c99c54dd8-77blm 6060:6060`

### Get running goroutines and heap
You can get the running goroutines and the heap allocations. This data can be used to locate possible memory leaks.
Using the option `-http=:9001` the pprof tool will open a webserver for the analysis.

`go tool pprof -http=:9001 localhost:6060/debug/pprof/goroutine`

`go tool pprof -http=:9002 localhost:6060/debug/pprof/heap`

### Get trace over 5s
You can get all calls that the server did and how long they took within x seconds using the trace. 

`curl -k http://localhost:6060/debug/pprof/trace\?debug\=1\&seconds\=5 -o cpu-trace.out`

This will create a `cpu-trace.out` file which you open within the webserver using command:

`go tool trace -http=:9003 ./cpu-trace.out`

### List all routines
Additionally, if you want a list of goroutines you can query it usign commnad:

`curl -k -o stage.out http://localhost:6060/debug/pprof/goroutine\?debug\=1`
