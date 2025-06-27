package policy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	liburl "net/url"
	"os"
	"time"

	refapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
)

var log = logging.WithName("validation|policy")

// Lib.
type LibClient = libweb.Client

// Application settings.
var Settings = &settings.Settings

// Pool (singleton).
var Agent Pool

// Error reported by the service.
type ValidationError struct {
	Errors []string
}

func (r *ValidationError) Error() string {
	return fmt.Sprintf("%v", r.Errors)
}

// Client.
type Client struct {
	LibClient
}

// Enabled.
func (r *Client) Enabled() bool {
	return Settings.PolicyAgent.Enabled()
}

// Policy version.
func (r *Client) Version(path string) (version int, err error) {
	if !r.Enabled() {
		return
	}
	out := &struct {
		Result struct {
			Version int `json:"rules_version"`
		} `json:"result"`
	}{}
	err = r.get(path, out)
	if err != nil {
		return
	}

	version = out.Result.Version

	log.V(3).Info(
		"Policy version detected.",
		"endpoint",
		path,
		"version",
		version)

	return
}

// Validate the VM.
func (r *Client) Validate(
	path string,
	workload interface{}) (version int, concerns []model.Concern, err error) {
	//
	if !r.Enabled() {
		return
	}
	in := &struct {
		Input interface{} `json:"input"`
	}{}
	in.Input = workload
	out := &struct {
		Result struct {
			Version  int             `json:"rules_version"`
			Concerns []model.Concern `json:"concerns"`
			Errors   []string        `json:"errors"`
		}
	}{}
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
	log.V(5).Info(
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
	log.V(5).Info(
		"POST request.",
		"url",
		url,
		"body",
		in)
	status, err := r.LibClient.Post(url, in, out)
	if err != nil {
		return
	}
	if status != http.StatusOK {
		err = liberr.New(http.StatusText(status))
	}

	return
}

// Build and set the transport as needed.
func (c *Client) buildTransport() (err error) {
	if c.Transport != nil || !Settings.PolicyAgent.Enabled() {
		return
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 10 * time.Second,
		}).DialContext,
		MaxIdleConns:          10,
		IdleConnTimeout:       10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	if len(Settings.PolicyAgent.TLS.CA) > 0 {
		pool := x509.NewCertPool()
		ca, xErr := os.ReadFile(Settings.PolicyAgent.TLS.CA)
		if xErr != nil {
			err = liberr.Wrap(xErr)
			return
		}
		pool.AppendCertsFromPEM(ca)
		transport.TLSClientConfig = &tls.Config{
			RootCAs: pool,
		}
	} else if Settings.Development {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	c.Transport = transport

	return
}

// Policy agent task.
type Task struct {
	// Path (endpoint).
	Path string
	// VM reference.
	Ref refapi.Ref
	// Revision number of the VM being validated.
	Revision int64
	// Context.
	Context context.Context
	// Workload builder.
	Workload func(string) (interface{}, error)
	// Task result channel.
	Result chan *Task
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

// Worker ID.
func (r *Task) Worker() int {
	return r.worker
}

// Duration.
func (r *Task) Duration() time.Duration {
	return r.completed.Sub(r.started)
}

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

// Notify result handler the task has completed.
func (r *Task) notify() {
	func() {
		if err := recover(); err != nil {
			log.Info("recovered from panic: ", "err", err)
		}
	}()
	if !r.canceled() {
		r.Result <- r
	}
}

// Task canceled.
func (r *Task) canceled() bool {
	select {
	case <-r.Context.Done():
		return true
	default:
		return false
	}
}

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

// Main worker run.
// Process input queue. Validation delegated to the
// policy agent.
func (r *Worker) run() {
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
			if task.canceled() {
				continue
			}
			task.worker = r.id
			task.started = time.Now()
			workload, err := task.Workload(task.Ref.ID)
			if err == nil {
				task.Version, task.Concerns, task.Error = r.client.Validate(task.Path, workload)
				task.completed = time.Now()
			} else {
				task.Error = err
				task.Concerns = []model.Concern{
					{
						Category:   "Critical",
						Label:      err.Error(),
						Assessment: err.Error(),
					},
				}
			}
			func() {
				defer func() {
					_ = recover()
				}()
				r.output <- task
			}()
		}
	}()
}

// Policy agent task pool.
type Pool struct {
	Client
	// Worker input queue.
	input chan *Task
	// Worker output queue.
	output chan *Task
	// Dispatcher has been started.  See: Run().
	started bool
}

// Main.
// Start workers and process output queue.
func (r *Pool) Start() {
	if r.started {
		return
	}
	r.output = make(chan *Task)
	r.input = make(chan *Task)
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
			"Pool started.")
		defer log.V(1).Info(
			"Pool stopped.")
		for task := range r.output {
			if task.Error == nil {
				log.V(4).Info(
					"VM validation succeeded.",
					"task",
					task.String())
			} else {
				log.Error(
					task.Error,
					"VM validation failed.",
					"task", task.String(),
				)
			}
			task.notify()
		}
	}()

	r.started = true
}

// Shutdown the pool.
// Terminate workers and stop processing result queue.
func (r *Pool) Shutdown() {
	if !r.started {
		return
	}
	r.started = false
	close(r.input)
	close(r.output)
}

// Policy version.
func (r *Pool) Version(path string) (version int, err error) {
	return r.Client.Version(path)
}

// Submit validation task.
// Queue validation request.
func (r *Pool) Submit(task *Task) (err error) {
	if !r.started {
		return liberr.New("pool not started.")
	}
	defer func() {
		_ = recover()
	}()
	r.input <- task
	return
}

// The pool backlog.
func (r *Pool) Backlog() int {
	return len(r.input)
}

// Number of workers.
func (r *Pool) parallel() (limit int) {
	limit = Settings.PolicyAgent.Limit.Worker
	if limit < 1 {
		limit = 1
	}

	return
}
