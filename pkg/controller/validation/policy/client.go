package policy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	"github.com/konveyor/controller/pkg/logging"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	refapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/ref"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/forklift-controller/pkg/settings"
	"io/ioutil"
	"net/http"
	liburl "net/url"
	"time"
)

var log = logging.WithName("validation|policy")

//
// Lib.
type LibClient = libweb.Client

//
// Application settings.
var Settings = &settings.Settings

//
// New policy agent.
func New(provider *api.Provider) *Scheduler {
	return &Scheduler{
		Client: Client{
			Provider: provider,
		},
	}
}

//
// Error reported by the service.
type ValidationError struct {
	Errors []string
}

func (r *ValidationError) Error() string {
	return fmt.Sprintf("%v", r.Errors)
}

//
// Client.
type Client struct {
	LibClient
	// Provider.
	Provider *api.Provider
}

//
// Enabled.
func (r *Client) Enabled() bool {
	return Settings.PolicyAgent.Enabled()
}

//
// Policy version.
func (r *Client) Version() (version int, err error) {
	out := &struct {
		Result struct {
			Version int `json:"rules_version"`
		} `json:"result"`
	}{}
	path := "/v1/data/io/konveyor/forklift/vmware/rules_version"
	err = r.get(path, out)
	if err != nil {
		return
	}

	version = out.Result.Version

	log.Info(
		"Policy version detected.",
		"version",
		version)

	return
}

//
// Validate the VM.
func (r *Client) Validate(ref refapi.Ref) (version int, concerns []model.Concern, err error) {
	if !r.Enabled() {
		return
	}
	in := &struct {
		Input struct {
			Provider struct {
				UID       string `json:"uid"`
				Namespace string `json:"namespace"`
				Name      string `json:"name"`
			} `json:"provider"`
			VmID string `json:"vm_moref"`
		} `json:"input"`
	}{}
	in.Input.Provider.UID = string(r.Provider.UID)
	in.Input.VmID = ref.ID
	out := &struct {
		Result struct {
			Version  int             `json:"rules_version"`
			Concerns []model.Concern `json:"concerns"`
			Errors   []string        `json:"errors"`
		}
	}{}
	path := "/v1/data/io/konveyor/forklift/vmware/validate"
	err = r.post(path, in, out)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(out.Result.Errors) > 0 {
		err = liberr.Wrap(
			&ValidationError{
				Errors: out.Result.Errors,
			})
		return
	}

	concerns = out.Result.Concerns
	version = out.Result.Version

	return
}

//
// Get request.
func (r *Client) get(path string, out interface{}) (err error) {
	parsedURL, err := liburl.Parse(Settings.PolicyAgent.URL)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	err = r.buildTransport()
	if err != nil {
		return
	}
	parsedURL.Path = path
	url := parsedURL.String()
	log.V(4).Info(
		"GET request.",
		"url",
		url)
	status, err := r.LibClient.Get(url, out)
	if err != nil {
		return
	}
	if status != http.StatusOK {
		err = liberr.New(http.StatusText(status))
	}

	return
}

//
// Post request.
func (r *Client) post(path string, in interface{}, out interface{}) (err error) {
	parsedURL, err := liburl.Parse(Settings.PolicyAgent.URL)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	err = r.buildTransport()
	if err != nil {
		return
	}
	parsedURL.Path = path
	url := parsedURL.String()
	log.V(4).Info(
		"POST request.",
		"url",
		url,
		"body",
		out)
	status, err := r.LibClient.Post(url, in, out)
	if err != nil {
		return
	}
	if status != http.StatusOK {
		err = liberr.New(http.StatusText(status))
	}

	return
}

//
// Build and set the transport as needed.
func (c *Client) buildTransport() (err error) {
	if c.Transport != nil {
		return
	}
	pool := x509.NewCertPool()
	ca, err := ioutil.ReadFile(Settings.PolicyAgent.CA)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	pool.AppendCertsFromPEM(ca)
	c.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: pool,
		},
	}

	return
}

//
// Dispatcher backlog (queue limit) exceeded.
type BacklogExceededError struct {
}

func (r BacklogExceededError) Error() string {
	return "Dispatcher backlog exceeded."
}

//
// Policy agent task.
type Task struct {
	// VM reference.
	Ref refapi.Ref
	// Revision number of the VM being validated.
	Revision interface{}
	// Result handler.
	ResultHandler func(*Task)
	// Reported policy version.
	Version int
	// Reported concerns.
	Concerns []model.Concern
	// Reported error.
	Error error
	// Worker ID.
	worker int
	// Started timestamp.
	started time.Time
	// Completed timestamp.
	completed time.Time
}

//
// Worker ID.
func (r *Task) Worker() int {
	return r.worker
}

//
// Duration.
func (r *Task) Duration() time.Duration {
	return r.completed.Sub(r.started)
}

//
// Description.
func (r *Task) String() string {
	err := ""
	if r.Error != nil {
		err = r.Error.Error()
	}
	return fmt.Sprintf(
		"Ref:%s,Version:%d,Error:'%s',Duration:%s,Concerns:%s",
		r.Ref.String(),
		r.Version,
		err,
		r.Duration(),
		r.Concerns)
}

//
// Notify result handler the task has completed.
func (r *Task) notify() {
	if r.ResultHandler != nil {
		r.ResultHandler(r)
	}
}

//
// Task worker.
type Worker struct {
	id int
	// Client.
	client Client
	// Input queue.
	input chan *Task
	// Output (result) queue.
	output chan *Task
}

//
// Main worker run.
// Process input queue. Validation delegated to the
// policy agent.
func (r *Worker) run() {
	defer func() {
		_ = recover()
	}()
	go func() {
		log.V(1).Info(
			"Worker started.",
			"id",
			r.id)
		defer log.V(1).Info(
			"Worker stopped.",
			"id",
			r.id)
		for task := range r.input {
			task.worker = r.id
			task.started = time.Now()
			task.Version, task.Concerns, task.Error = r.client.Validate(task.Ref)
			task.completed = time.Now()
			r.output <- task
		}
	}()
}

//
// Policy agent task scheduler.
type Scheduler struct {
	Client
	// Worker input queue.
	input chan *Task
	// Worker output queue.
	output chan *Task
	// Dispatcher has been started.  See: Run().
	started bool
}

//
// Main.
// Start workers and process output queue.
func (r *Scheduler) Start() {
	if r.started {
		return
	}
	r.input = make(chan *Task, r.backlog())
	r.output = make(chan *Task)
	for id := 0; id < r.parallel(); id++ {
		w := Worker{
			id:     id,
			client: r.Client,
			input:  r.input,
			output: r.output,
		}
		w.run()
	}
	go func() {
		log.V(1).Info(
			"Scheduler started.")
		defer log.V(1).Info(
			"Scheduler stopped.")
		for task := range r.output {
			if task.Error == nil {
				log.V(4).Info(
					"VM Validation succeeded.",
					"task",
					task.String())
			} else {
				log.Info(
					"VM Validation failed.",
					"task",
					task)
			}
			task.notify()
		}
	}()

	r.started = true
}

//
// Shutdown the scheduler.
// Terminate workers and stop processing result queue.
func (r *Scheduler) Shutdown() {
	if !r.started {
		return
	}
	r.started = false
	close(r.input)
	close(r.output)
}

//
// Policy version.
func (r *Scheduler) Version() (version int, err error) {
	return r.Client.Version()
}

//
// Submit validation task.
// Queue validation request.
// May block (no longer than 10 seconds) when backlog exceeded.
// Returns: BacklogExceededError.
func (r *Scheduler) Submit(task *Task) (err error) {
	if !r.started {
		return liberr.New("scheduler not started.")
	}
	defer func() {
		_ = recover()
	}()
	select {
	case r.input <- task:
		// queued.
	case <-time.After(10 * time.Minute):
		err = liberr.Wrap(BacklogExceededError{})
	}

	return
}

//
// Backlog limit.
// Input queue depth.
func (r *Scheduler) backlog() (limit int) {
	limit = Settings.PolicyAgent.Limit.Backlog
	if limit < 1 {
		limit = 1
	}

	return
}

//
// Number of workers.
func (r *Scheduler) parallel() (limit int) {
	limit = Settings.PolicyAgent.Limit.Worker
	if limit < 1 {
		limit = 1
	}

	return
}
