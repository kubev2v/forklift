package plan

import meta "k8s.io/apimachinery/pkg/apis/meta/v1"

// Resources that record started and completed timestamps.
type Timed struct {
	// Started timestamp.
	Started *meta.Time `json:"started,omitempty"`
	// Completed timestamp.
	Completed *meta.Time `json:"completed,omitempty"`
}

// Reset.
func (r *Timed) MarkReset() {
	r.Started = nil
	r.Completed = nil
}

// Mark as started.
func (r *Timed) MarkStarted() {
	if r.Started == nil {
		r.Started = r.now()
		r.Completed = nil
	}
}

// Mark as completed.
func (r *Timed) MarkCompleted() {
	r.MarkStarted()
	if r.Completed == nil {
		r.Completed = r.now()
	}
}

// Has started.
func (r *Timed) MarkedStarted() bool {
	return r.Started != nil
}

// Is migrating.
func (r *Timed) Running() bool {
	return r.MarkedStarted() && !r.MarkedCompleted()
}

// Has completed.
func (r *Timed) MarkedCompleted() bool {
	return r.Completed != nil
}

func (r *Timed) now() *meta.Time {
	now := meta.Now()
	return &now
}
