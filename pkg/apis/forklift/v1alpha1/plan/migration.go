package plan

import (
	libitr "github.com/konveyor/controller/pkg/itinerary"
	"k8s.io/apimachinery/pkg/types"
)

//
// Error.
type Error struct {
	Phase   string   `json:"phase"`
	Reasons []string `json:"reasons"`
}

//
// Add.
func (e *Error) Add(reason ...string) {
	find := func(reason string) (found bool) {
		for _, r := range e.Reasons {
			if r == reason {
				found = true
				break
			}
		}
		return
	}
	for _, r := range reason {
		if !find(r) {
			e.Reasons = append(e.Reasons, r)
		}
	}
}

//
// Migration status.
type MigrationStatus struct {
	Timed `json:",inline"`
	// Active migration.
	Active types.UID `json:"active"`
	// VM status
	VMs []*VMStatus `json:"vms,omitempty"`
}

//
// Pipeline step.
type Step struct {
	Task `json:",inline"`
	// Nested tasks.
	Tasks []*Task `json:"tasks,omitempty"`
}

//
// Find task by name.
func (r *Step) FindTask(name string) (task *Task, found bool) {
	for _, task = range r.Tasks {
		if task.Name == name {
			found = true
			break
		}
	}

	return
}

//
// Reflect task progress and errors.
func (r *Step) ReflectTasks() {
	tasksStarted := 0
	tasksCompleted := 0
	completed := int64(0)
	if len(r.Tasks) == 0 {
		return
	}
	for _, task := range r.Tasks {
		if task.MarkedStarted() {
			tasksStarted++
		}
		if task.MarkedCompleted() {
			tasksCompleted++
		}
		completed += task.Progress.Completed
		if task.Error != nil {
			task.AddError(task.Error.Reasons...)
		}
	}
	r.Progress.Completed = completed
	if tasksStarted > 0 {
		r.MarkStarted()
	}
	if tasksCompleted == len(r.Tasks) {
		r.MarkCompleted()
	}
}

//
// Migration task.
type Task struct {
	Timed `json:",inline"`
	// Name.
	Name string `json:"name"`
	// Name
	Description string `json:"description,omitempty"`
	// Phase
	Phase string `json:"phase,omitempty"`
	// Progress.
	Progress libitr.Progress `json:"progress"`
	// Annotations.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Error.
	Error *Error `json:"error,omitempty"`
}

//
// Add an error.
func (r *Task) AddError(reason ...string) {
	if r.Error == nil {
		r.Error = &Error{Phase: r.Phase}
	}
	r.Error.Add(reason...)
}
