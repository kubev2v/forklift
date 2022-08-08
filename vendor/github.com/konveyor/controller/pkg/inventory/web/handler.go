package web

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/gorilla/websocket"
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/controller/pkg/inventory/container"
	"github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/controller/pkg/ref"
	"net/http"
	"strconv"
	"time"
)

//
// Web request handler.
type RequestHandler interface {
	// Add routes to the `gin` router.
	AddRoutes(*gin.Engine)
}

//
// Paged handler.
type Paged struct {
	// The `page` parameter passed in the request.
	Page model.Page
}

//
// Prepare the handler to fulfil the request.
// Set the `page` field using passed parameters.
func (h *Paged) Prepare(ctx *gin.Context) int {
	status := h.setPage(ctx)
	if status != http.StatusOK {
		return status
	}

	return http.StatusOK
}

//
// Set the `page` field.
func (h *Paged) setPage(ctx *gin.Context) int {
	q := ctx.Request.URL.Query()
	page := model.Page{
		Limit:  int(^uint(0) >> 1),
		Offset: 0,
	}
	pLimit := q.Get("limit")
	if len(pLimit) != 0 {
		nLimit, err := strconv.Atoi(pLimit)
		if err != nil || nLimit < 0 {
			return http.StatusBadRequest
		}
		page.Limit = nLimit
	}
	pOffset := q.Get("offset")
	if len(pOffset) != 0 {
		nOffset, err := strconv.Atoi(pOffset)
		if err != nil || nOffset < 0 {
			return http.StatusBadRequest
		}
		page.Offset = nOffset
	}

	h.Page = page
	return http.StatusOK
}

//
// Parity (not-partial) request handler.
type Parity struct {
}

//
// Ensure collector has achieved parity.
func (c *Parity) EnsureParity(r container.Collector, w time.Duration) int {
	wait := w
	poll := time.Microsecond * 100
	for {
		mark := time.Now()
		if r.HasParity() {
			return http.StatusOK
		}
		if wait > 0 {
			time.Sleep(poll)
			wait -= time.Since(mark)
		} else {
			break
		}
	}

	return http.StatusPartialContent
}

//
// Watched resource builder.
type ResourceBuilder func(model.Model) interface{}

//
// Event
type Event struct {
	// ID
	ID uint64
	// Labels.
	Labels []string
	// Action.
	Action uint8
	// Affected Resource.
	Resource interface{}
	// Updated resource.
	Updated interface{}
}

//
// String representation.
func (r *Event) String() string {
	action := "unknown"
	switch r.Action {
	case model.Started:
		action = "started"
	case model.Parity:
		action = "parity"
	case model.Error:
		action = "error"
	case model.End:
		action = "end"
	case model.Created:
		action = "created"
	case model.Updated:
		action = "updated"
	case model.Deleted:
		action = "deleted"
	}
	kind := ""
	if r.Resource != nil {
		kind = ref.ToKind(r.Resource)
	}
	return fmt.Sprintf(
		"event-%.4d: %s kind=%s",
		r.ID,
		action,
		kind)
}

//
// Watch (event) writer.
// The writer is model event handler. Each event
// is send (forwarded) to the watch client.  This
// provides the bridge between the model and web layer.
type WatchWriter struct {
	// Watch options.
	options model.WatchOptions
	// Negotiated web socket.
	webSocket *websocket.Conn
	// Resource.
	builder ResourceBuilder
	// Logger.
	log logr.Logger
	// Done.
	done bool
}

//
// Watch options.
func (r *WatchWriter) Options() model.WatchOptions {
	return r.options
}

//
// Start the writer.
// Detect connection closed by peer or broken
// and end the watch.
func (r *WatchWriter) Start(watch *model.Watch) {
	go func() {
		time.Sleep(time.Second)
		defer func() {
			r.log.V(3).Info("stopped.")
		}()
		for {
			event := Event{}
			err := r.webSocket.ReadJSON(&event)
			if r.done {
				return
			}
			if err != nil {
				r.log.V(4).Info(err.Error())
				watch.End()
				return
			}
			switch event.Action {
			case model.End:
				r.log.V(4).Info("ended by peer.")
				watch.End()
				return
			}
		}
	}()
}

//
// Watch has started.
func (r *WatchWriter) Started(watchID uint64) {
	r.log.V(3).Info("event: started.")
	r.send(model.Event{
		ID:     watchID, // send watch ID.
		Action: model.Started,
	})
}

//
// Watch has parity.
func (r *WatchWriter) Parity() {
	r.log.V(3).Info("event: parity.")
	r.send(model.Event{
		Action: model.Parity,
	})
}

//
// A model has been created.
func (r *WatchWriter) Created(event model.Event) {
	r.log.V(5).Info(
		"event received.",
		"event",
		event.String())
	r.send(event)
}

//
// A model has been updated.
func (r *WatchWriter) Updated(event model.Event) {
	r.log.V(5).Info(
		"event received.",
		"event",
		event.String())
	r.send(event)
}

//
// A model has been deleted.
func (r *WatchWriter) Deleted(event model.Event) {
	r.log.V(5).Info(
		"event received.",
		"event",
		event.String())
	r.send(event)
}

//
// An error has occurred delivering an event.
func (r *WatchWriter) Error(err error) {
	r.log.V(3).Info(
		"event: error",
		"error",
		err.Error())
	r.send(model.Event{
		Action: model.Error,
	})
}

//
// An event watch has ended.
func (r *WatchWriter) End() {
	r.log.V(3).Info("event: ended.")
	r.send(model.Event{
		Action: model.End,
	})
	r.done = true
	time.Sleep(50 * time.Millisecond)
	_ = r.webSocket.Close()
}

//
// Write event to the socket.
func (r *WatchWriter) send(e model.Event) {
	if r.done {
		return
	}
	event := Event{
		ID:     e.ID,
		Labels: e.Labels,
		Action: e.Action,
	}
	if e.Model != nil {
		event.Resource = r.builder(e.Model)
	}
	if e.Updated != nil {
		event.Updated = r.builder(e.Updated)
	}
	err := r.webSocket.WriteJSON(event)
	if err != nil {
		r.log.V(4).Error(err, "websocket send failed.")
	}

	r.log.V(5).Info(
		"event sent.",
		"event",
		event)
}

//
// Watched (handler).
type Watched struct {
	// Watch requested.
	WatchRequest bool
	// Watch options.
	options model.WatchOptions
}

//
// Prepare the handler to fulfil the request.
// Set the `WatchRequest` and `snapshot` fields based on passed headers.
// The header value is a list of options.
func (h *Watched) Prepare(ctx *gin.Context) int {
	header, found := ctx.Request.Header[WatchHeader]
	h.WatchRequest = found
	for _, option := range header {
		switch option {
		case WatchSnapshot:
			h.options.Snapshot = true
		}
	}

	return http.StatusOK
}

//
// Watch model.
func (r *Watched) Watch(
	ctx *gin.Context,
	db model.DB,
	m model.Model,
	rb ResourceBuilder) (err error) {
	//
	upGrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	socket, err := upGrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		err = liberr.Wrap(
			err,
			"websocket upgrade failed.",
			"url",
			ctx.Request.URL)
		return
	}
	name := "web|watch|writer"
	writer := &WatchWriter{
		options:   r.options,
		webSocket: socket,
		builder:   rb,
		log: logging.WithName(name).WithValues(
			"peer",
			socket.RemoteAddr()),
	}
	watch, err := db.Watch(m, writer)
	if err != nil {
		_ = socket.Close()
		return
	}
	writer.log = logging.WithName(name).WithValues(
		"peer",
		socket.RemoteAddr(),
		"watch",
		watch.String())

	writer.Start(watch)

	log.V(3).Info(
		"handler: watch created.",
		"url",
		ctx.Request.URL,
		"watch",
		watch.String())

	return
}

//
// Schema (route) handler.
type SchemaHandler struct {
	// The `gin` router.
	router *gin.Engine
	// Schema version
	Version string
	// Schema release.
	Release int
}

//
// Add routes.
func (h *SchemaHandler) AddRoutes(r *gin.Engine) {
	r.GET("/schema", h.List)
	h.router = r
}

//
// List schema.
func (h *SchemaHandler) List(ctx *gin.Context) {
	type Schema struct {
		Version string   `json:"version,omitempty"`
		Release int      `json:"release,omitempty"`
		Paths   []string `json:"paths"`
	}
	schema := Schema{
		Version: h.Version,
		Release: h.Release,
		Paths:   []string{},
	}
	for _, rte := range h.router.Routes() {
		schema.Paths = append(schema.Paths, rte.Path)
	}

	ctx.JSON(http.StatusOK, schema)
}

//
// Not supported.
func (h SchemaHandler) Get(ctx *gin.Context) {
	ctx.Status(http.StatusMethodNotAllowed)
}
