package catalog

import "sync"

// NewQueuingWaitGroup creates a QueuingWaitGroup with the
// requested limit.
func NewQueuingWaitGroup(limit int) *QueuingWaitGroup {
	q := &QueuingWaitGroup{}
	q.Reset(limit)
	return q
}

// QueuingWaitGroup is a wait group that only
// permits a limited number of callers to wait
// at once.
type QueuingWaitGroup struct {
	limit chan int
	wg    sync.WaitGroup
}

func (r *QueuingWaitGroup) Reset(limit int) {
	r.limit = make(chan int, limit)
	r.wg = sync.WaitGroup{}
}

func (r *QueuingWaitGroup) Add() {
	r.limit <- 1
	r.wg.Add(1)
}

func (r *QueuingWaitGroup) Done() {
	<-r.limit
	r.wg.Done()
}

func (r *QueuingWaitGroup) Wait() {
	r.wg.Wait()
}
