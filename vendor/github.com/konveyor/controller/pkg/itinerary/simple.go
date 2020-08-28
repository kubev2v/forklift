package itinerary

import liberr "github.com/konveyor/controller/pkg/error"

//
// List of steps.
type Pipeline []Step

//
// Predicate flag.
type Flag = int16

//
// Predicate.
// Flags delegated to the predicate.
type Predicate interface {
	// Evaluate the condition.
	// Returns (true) when the step should be included.
	Evaluate(Flag) (bool, error)
}

//
// Itinerary step.
type Step struct {
	// Name.
	Name string
	// Any of these conditions be satisfied for
	// the step to be included.
	All Flag
	// All of these conditions be satisfied for
	// the step to be included.
	Any Flag
}

//
// An itinerary.
// List of conditional steps.
type Itinerary struct {
	// Pipeline (list) of steps.
	Pipeline
	// Predicate.
	Predicate
	// Name.
	Name string
}

//
// Errors.
var (
	StepNotFound = liberr.New("step not found")
)

//
// Get a step by name.
func (r *Itinerary) Get(name string) (step Step, err error) {
	for _, step = range r.Pipeline {
		if step.Name == name {
			return
		}
	}

	err = StepNotFound
	return
}

//
// Get the first step filtered by predicate.
func (r *Itinerary) First() (step Step, err error) {
	list, pErr := r.List()
	if pErr != nil {
		err = liberr.Wrap(pErr)
		return
	}
	if len(list) > 0 {
		step = list[0]
	} else {
		err = StepNotFound
	}

	return
}

// List of steps filtered by predicates.
func (r *Itinerary) List() (pipeline Pipeline, err error) {
	for _, step := range r.Pipeline {
		pTrue, pErr := r.hasAny(step)
		if pErr != nil {
			err = liberr.Wrap(pErr)
			return
		}
		if !pTrue {
			continue
		}
		pTrue, pErr = r.hasAll(step)
		if pErr != nil {
			err = liberr.Wrap(pErr)
			return
		}
		if !pTrue {
			continue
		}

		pipeline = append(pipeline, step)
	}

	return
}

//
// Get the next step in the itinerary.
func (r *Itinerary) Next(name string) (next Step, done bool, err error) {
	current, pErr := r.Get(name)
	if pErr != nil {
		err = liberr.Wrap(pErr)
		return
	}
	list, pErr := r.List()
	if pErr != nil {
		err = liberr.Wrap(pErr)
		return
	}
	matched := false
	for _, step := range list {
		if matched {
			next = step
			return
		}
		if step.Name == current.Name {
			matched = true
		}
	}

	done = true
	return
}

// Build a progress report.
func (r *Itinerary) Progress(step string) (report Progress, err error) {
	list, err := r.List()
	if err != nil {
		return
	}
	report.Total = len(list)
	for _, s := range list {
		if s.Name != step {
			report.Completed++
		} else {
			break
		}
	}

	return
}

//
// The step has satisfied ANY of the predicates.
func (r *Itinerary) hasAny(step Step) (pTrue bool, err error) {
	for i := 0; i < 16; i++ {
		flag := Flag(1 << i)
		if (step.Any & flag) == 0 {
			continue
		}
		if r.Predicate == nil {
			continue
		}
		pTrue, err = r.Predicate.Evaluate(flag)
		if pTrue || err != nil {
			return
		}
	}

	pTrue = true
	return
}

//
// The step has satisfied ALL of the predicates.
func (r *Itinerary) hasAll(step Step) (pTrue bool, err error) {
	for i := 0; i < 16; i++ {
		flag := Flag(1 << i)
		if (step.All & flag) == 0 {
			continue
		}
		if r.Predicate == nil {
			continue
		}
		pTrue, err = r.Predicate.Evaluate(flag)
		if !pTrue || err != nil {
			return
		}
	}

	pTrue = true
	return
}

//
// Progress report.
type Progress struct {
	// Completed units.
	Completed int `json:"completed"`
	// Total units.
	Total int `json:"total"`
}
