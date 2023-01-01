// Web stack integration test.
package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	"github.com/konveyor/forklift-controller/pkg/lib/inventory/container"
	"github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
	"github.com/konveyor/forklift-controller/pkg/lib/inventory/web"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var log = logging.WithName("TESTER")

// Model object.
type Model struct {
	ID   int    `sql:"pk"`
	Name string `sql:"index(a)"`
	Age  int    `sql:"index(a)"`
}

func (m *Model) Pk() string {
	return fmt.Sprintf("%d", m.ID)
}

func (m *Model) String() string {
	return fmt.Sprintf(
		"Model: id: %d, name:%s",
		m.ID,
		m.Name)
}

func (m *Model) Equals(other model.Model) bool {
	return false
}

func (m *Model) Labels() model.Labels {
	return nil
}

// Watch (event) handler.
type EventHandler struct {
	options web.WatchOptions
	name    string
	started bool
	parity  bool
	created []int
	updated []int
	deleted []int
	err     []error
	done    bool
	wid     uint64
}

func (h *EventHandler) Options() web.WatchOptions {
	return h.options
}

func (h *EventHandler) Started(wid uint64) {
	h.wid = wid
	h.started = true

	fmt.Printf("[%d] Event (started)\n", wid)
}

func (h *EventHandler) Parity() {
	h.parity = true

	fmt.Printf("[%d] Event (parity)\n", h.wid)
}

func (h *EventHandler) Created(e web.Event) {
	if object, cast := e.Resource.(*Model); cast {
		h.created = append(h.created, object.ID)
	}

	fmt.Printf("[%d] Event (created): %v\n", h.wid, e)
}

func (h *EventHandler) Updated(e web.Event) {
	if object, cast := e.Resource.(*Model); cast {
		h.updated = append(h.updated, object.ID)
	}

	fmt.Printf("[%d] Event (updated): %v\n", h.wid, e)
}
func (h *EventHandler) Deleted(e web.Event) {
	if object, cast := e.Resource.(*Model); cast {
		h.deleted = append(h.deleted, object.ID)
	}

	fmt.Printf("[%d] Event (deleted): %v\n", h.wid, e)
}

func (h *EventHandler) Error(w *web.Watch, err error) {
	h.err = append(h.err, err)
	_ = w.Repair()

	fmt.Printf("[%d] Event (error): %v\n", h.wid, err)
}

func (h *EventHandler) End() {
	h.done = true

	fmt.Printf("[%d] Event (end)\n", h.wid)
}

type Endpoint struct {
	web.Watched
	db model.DB
}

func (h Endpoint) Get(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.Query("id"))
	m := &Model{ID: id}
	err := h.db.Get(m)
	if err != nil {
		if errors.Is(err, model.NotFound) {
			ctx.Status(http.StatusNotFound)
		} else {
			ctx.Status(http.StatusInternalServerError)
		}
		return
	}

	ctx.JSON(http.StatusOK, m)
}

func (h Endpoint) List(ctx *gin.Context) {
	// Watch request.
	h.Watched.Prepare(ctx)
	if h.WatchRequest {
		err := h.Watch(
			ctx,
			h.db,
			&Model{},
			func(in model.Model) (r interface{}) {
				r = in
				return
			})
		if err != nil {
			ctx.Status(http.StatusInternalServerError)
		}
		return
	}
	// List request.
	list := []Model{}
	err := h.db.List(&list, model.ListOptions{Detail: model.MaxDetail})
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, list)
}

func (h *Endpoint) AddRoutes(e *gin.Engine) {
	e.GET("/models", h.List)
	e.GET("/models/:id", h.Get)
}

// Data collector.
type Collector struct {
	db model.DB
}

func (r *Collector) Name() string {
	return "tester"
}

func (r *Collector) Owner() meta.Object {
	return &meta.ObjectMeta{
		UID: "TEST",
	}
}

func (r *Collector) Start() error {
	return nil
}

func (r *Collector) Shutdown() {
}

func (r *Collector) DB() model.DB {
	return r.db
}

func (r *Collector) HasParity() bool {
	return true
}

func (r *Collector) Test() (int, error) {
	return 0, nil
}

func (r *Collector) Reset() {
}

func setup() (db model.DB, webSrv *web.WebServer) {
	//
	// open DB.
	db = model.New("/tmp/integration.db", &Model{})
	err := db.Open(true)
	if err != nil {
		panic(err)
	}
	//
	// Populate the DB.
	for i := 0; i < 10; i++ {
		err = db.Insert(
			&Model{
				ID:   i,
				Name: fmt.Sprintf("m-%.4d", i),
				Age:  i + 10,
			})
		if err != nil {
			panic(err)
		}
	}

	//
	// Build container.
	dr := &Collector{db}
	cnt := container.New()
	err = cnt.Add(dr)
	if err != nil {
		panic(err)
	}

	//
	// Launch web server.
	webSrv = web.New(cnt, &Endpoint{db: db})
	webSrv.Port = 7001
	webSrv.Start()
	return
}

func list(client *web.Client) {
	list := []Model{}
	status, err := client.Get("http://localhost:7001/models", &list)
	if err != nil {
		panic(err)
	}
	if status != http.StatusOK {
		panic(liberr.New(http.StatusText(status)))
	}

	fmt.Println("List")
	fmt.Println("___________________________")
	for _, m := range list {
		fmt.Println(m)
	}
}

func get(client *web.Client) {
	m := &Model{}
	status, err := client.Get("http://localhost:7001/models/0", m)
	if err != nil {
		panic(err)
	}
	if status != http.StatusOK {
		panic(liberr.New(http.StatusText(status)))
	}

	fmt.Printf("\nGet: %v\n", m)
}

func watch(client *web.Client, snapshot bool) (watch *web.Watch) {
	status, watch, err := client.Watch(
		"http://localhost:7001/models",
		&Model{},
		&EventHandler{
			options: web.WatchOptions{
				Snapshot: snapshot,
			},
		})
	if err != nil {
		panic(err)
	}
	if status != http.StatusOK {
		panic(liberr.New(http.StatusText(status)))
	}

	fmt.Printf("\nWatch started: %d  (snapshot=%v)\n", watch.ID(), snapshot)

	return
}

func endWatch(w *web.Watch) {
	fmt.Printf("\nEnd watch: %d\n", w.ID())
	w.End()
	wait(500)
}

func wait(d time.Duration) {
	time.Sleep(d * time.Millisecond)
}

// Basic test.
func testA(client *web.Client) {
	w := watch(client, true)
	get(client)
	list(client)
	endWatch(w)
	wait(500)
}

// Test client watch normal lifecycle.
func testB(client *web.Client, n int) {
	for i := 0; i < n; i++ {
		w := watch(client, true)
		wait(500)
		endWatch(w)
	}
}

// Test watch client finalizer.
func testC(client *web.Client, n int) {
	for i := 0; i < n; i++ {
		w := watch(client, true)
		fmt.Println(w.ID())
		wait(500)
		w = nil
		runtime.GC()
		wait(500)
	}
}

// Watch no snapshot
func testD(db model.DB, client *web.Client) {
	w := watch(client, false)
	for i := 20; i < 25; i++ {
		err := db.Insert(
			&Model{
				ID:   i,
				Name: fmt.Sprintf("m-%.4d", i),
				Age:  i + 10,
			})
		if err != nil {
			panic(err)
		}
	}
	wait(100)
	endWatch(w)
	wait(500)
}

// Test close Db.
func testE(db model.DB, client *web.Client) {
	w := watch(client, true)
	wait(100)
	fmt.Println(w.ID())
	_ = db.Close(false)
}

// Main.
func main() {
	db, _ := setup()
	fmt.Println(db)
	client := &web.Client{
		Transport: http.DefaultTransport,
	}
	n := 3
	if len(os.Args) > 1 {
		n, _ = strconv.Atoi(os.Args[1])
	}
	wait(100)
	testA(client)
	testB(client, n)
	testC(client, n)
	testD(db, client)
	testE(db, client)
	wait(500)
}
