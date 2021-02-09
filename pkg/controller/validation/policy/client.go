package policy

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	refapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/ref"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/forklift-controller/pkg/settings"
	"io/ioutil"
	"net/http"
	liburl "net/url"
	"time"
)

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
	// Provider.
	Provider *api.Provider
	// Transport.
	transport *http.Transport
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
				Namespace string `json:"namespace"`
				Name      string `json:"name"`
			} `json:"provider"`
			ID string `json:"vm_moref"`
		} `json:"input"`
	}{}
	in.Input.Provider.Namespace = r.Provider.Namespace
	in.Input.Provider.Name = r.Provider.Name
	in.Input.ID = ref.ID
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
	parsedURL.Path = path
	request := &http.Request{
		Method: http.MethodGet,
		URL:    parsedURL,
	}
	err = r.buildTransport()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	client := http.Client{Transport: r.transport}
	response, err := client.Do(request)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	status := response.StatusCode
	content := []byte{}
	if status == http.StatusOK {
		defer response.Body.Close()
		content, err = ioutil.ReadAll(response.Body)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		err = json.Unmarshal(content, out)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	} else {
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
	parsedURL.Path = path
	body, _ := json.Marshal(in)
	reader := bytes.NewReader(body)
	request := &http.Request{
		Method: http.MethodPost,
		Body:   ioutil.NopCloser(reader),
		URL:    parsedURL,
	}
	err = r.buildTransport()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	client := http.Client{Transport: r.transport}
	response, err := client.Do(request)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	status := response.StatusCode
	content := []byte{}
	if status == http.StatusOK {
		defer response.Body.Close()
		content, err = ioutil.ReadAll(response.Body)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		err = json.Unmarshal(content, out)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	} else {
		err = liberr.New(http.StatusText(status))
	}

	return
}

//
// Build and set the transport as needed.
func (c *Client) buildTransport() (err error) {
	if c.transport != nil {
		return
	}
	pool := x509.NewCertPool()
	ca, err := ioutil.ReadFile(Settings.PolicyAgent.CA)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	pool.AppendCertsFromPEM(ca)
	c.transport = &http.Transport{
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
		for task := range r.output {
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
	case <-time.After(10 * time.Second):
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
