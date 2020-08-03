package logging

import (
	"errors"
	"github.com/go-logr/logr"
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/onsi/gomega"
	"testing"
)

type entry struct {
	message string
	kvpair  []interface{}
	err     error
}

type fake struct {
	entry []entry
}

func (l *fake) Info(message string, kvpair ...interface{}) {
	l.entry = append(
		l.entry,
		entry{
			message: message,
			kvpair:  kvpair,
		})
}

func (l *fake) Error(err error, message string, kvpair ...interface{}) {
	l.entry = append(
		l.entry,
		entry{
			message: message,
			kvpair:  kvpair,
			err:     err,
		})
}

func (l fake) Enabled() bool {
	return true
}

func (l fake) V(level int) logr.InfoLogger {
	return nil
}

func (l fake) WithName(name string) logr.Logger {
	return nil
}

//
// Get logger with values.
func (l fake) WithValues(kvpair ...interface{}) logr.Logger {
	return nil
}

func TestLogger(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	//
	// Real
	log := WithName("Test")
	log.Info("hello")
	log.Error(errors.New("A"), "thing failed")
	log.Trace(errors.New("B"))
	//
	// Faked
	log.Reset()
	f := &fake{entry: []entry{}}
	log.Real = f
	// Info
	log.Info("hello")
	g.Expect(len(f.entry)).To(gomega.Equal(1))
	g.Expect(len(f.entry[0].kvpair)).To(gomega.Equal(0))
	// Error
	log.Error(errors.New("C"), "thing failed")
	g.Expect(len(f.entry)).To(gomega.Equal(2))
	g.Expect(len(f.entry[1].kvpair)).To(gomega.Equal(0))
	// nil Error
	log.Error(nil, "thing failed")
	g.Expect(len(f.entry)).To(gomega.Equal(2))
	g.Expect(len(f.entry[1].kvpair)).To(gomega.Equal(0))
	// Trace
	log.Trace(errors.New("D"))
	g.Expect(len(f.entry)).To(gomega.Equal(3))
	g.Expect(len(f.entry[2].kvpair)).To(gomega.Equal(0))
	// Error (wrapped)
	log.Error(liberr.Wrap(errors.New("C wrapped")), "thing failed")
	g.Expect(len(f.entry)).To(gomega.Equal(4))
	g.Expect(len(f.entry[3].kvpair)).To(gomega.Equal(4))
	g.Expect(f.entry[3].kvpair[0]).To(gomega.Equal(Error))
	g.Expect(f.entry[3].kvpair[2]).To(gomega.Equal(Stack))
	// Trace (wrapped)
	log.Trace(liberr.Wrap(errors.New("D wrapped")))
	g.Expect(len(f.entry)).To(gomega.Equal(5))
	g.Expect(len(f.entry[4].kvpair)).To(gomega.Equal(4))
	g.Expect(f.entry[4].kvpair[0]).To(gomega.Equal(Error))
	g.Expect(f.entry[4].kvpair[2]).To(gomega.Equal(Stack))
}
