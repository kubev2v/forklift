package logging

import (
	"errors"
	"github.com/go-logr/logr"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	"github.com/onsi/gomega"
	"testing"
)

type entry struct {
	message string
	kvpair  []interface{}
	err     error
}

type fakeBuilder struct {
}

func (b *fakeBuilder) New() logr.Logger {
	return &fake{
		entry: []entry{},
	}
}

func (b *fakeBuilder) V(level int, f logr.Logger) logr.Logger {
	return &fake{
		debug: Settings.atDebug(level),
		entry: []entry{},
	}
}

type fake struct {
	debug  bool
	entry  []entry
	values []interface{}
	name   string
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

func (l *fake) Enabled() bool {
	return true
}

func (l *fake) V(level int) logr.InfoLogger {
	return &fake{
		entry: []entry{},
	}
}

func (l *fake) WithName(name string) logr.Logger {
	l.name = name
	return l
}

// Get logger with values.
func (l *fake) WithValues(kvpair ...interface{}) logr.Logger {
	l.values = kvpair
	return l
}

func TestReal(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	//
	// Real
	log := WithName("Test")
	log.Info("hello")
	log.Error(errors.New("A"), "thing failed")
	log.Trace(errors.New("B"))
	g.Expect(log.name).To(gomega.Equal("Test"))
}

func TestFake(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	Factory = &fakeBuilder{}

	log := WithName("Test")
	f := log.Real.(*fake)
	g.Expect(f.name).To(gomega.Equal("Test"))
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
	// Trace (wrapped) with context.
	log.Trace(
		liberr.Wrap(
			errors.New("D wrapped"),
			"Failed to create user.",
			"name", "larry",
			"age", 10),
		"a", "A",
		"b", "B")
	g.Expect(len(f.entry)).To(gomega.Equal(6))
	g.Expect(len(f.entry[5].kvpair)).To(gomega.Equal(12))
	g.Expect(f.entry[5].kvpair[0]).To(gomega.Equal("name"))
	g.Expect(f.entry[5].kvpair[1]).To(gomega.Equal("larry"))
	g.Expect(f.entry[5].kvpair[2]).To(gomega.Equal("age"))
	g.Expect(f.entry[5].kvpair[3]).To(gomega.Equal(10))
	g.Expect(f.entry[5].kvpair[4]).To(gomega.Equal("a"))
	g.Expect(f.entry[5].kvpair[5]).To(gomega.Equal("A"))
	g.Expect(f.entry[5].kvpair[6]).To(gomega.Equal("b"))
	g.Expect(f.entry[5].kvpair[7]).To(gomega.Equal("B"))
	g.Expect(f.entry[5].kvpair[8]).To(gomega.Equal(Error))
	g.Expect(f.entry[5].kvpair[10]).To(gomega.Equal(Stack))

	// Levels.
	// level-1
	Settings.Level = 0
	log = WithName("level-testing")
	log0 := log.V(0)
	g.Expect(log0.(*Logger).Real.(*fake).debug).To(gomega.BeFalse())
	log0.Info("Test-0")
	g.Expect(len(log0.(*Logger).Real.(*fake).entry)).To(gomega.Equal(1))
	log1 := log.V(1)
	log1.Info("Test-1")
	g.Expect(len(log1.(*Logger).Real.(*fake).entry)).To(gomega.Equal(0))
	// level-4
	Settings.Level = Settings.DebugThreshold
	log = WithName("level-testing")
	log0 = log.V(2)
	g.Expect(log0.(*Logger).Real.(*fake).debug).To(gomega.BeFalse())
	log0.Info("Test-0")
	g.Expect(len(log0.(*Logger).Real.(*fake).entry)).To(gomega.Equal(1))
	log1 = log.V(Settings.DebugThreshold)
	log1.Info("Test-1")
	g.Expect(len(log1.(*Logger).Real.(*fake).entry)).To(gomega.Equal(1))
	log2 := log1.V(Settings.DebugThreshold + 1)
	g.Expect(log2.(*Logger).Real.(*fake).debug).To(gomega.BeTrue())
	log2.Info("Test-2")
	g.Expect(len(log2.(*Logger).Real.(*fake).entry)).To(gomega.Equal(0))

	// level-1
	err := liberr.New("")
	Settings.Level = 0
	log = WithName("level-testing")
	log0 = log.V(0)
	g.Expect(log0.(*Logger).Real.(*fake).debug).To(gomega.BeFalse())
	log0.Error(err, "Test-0")
	g.Expect(len(log0.(*Logger).Real.(*fake).entry)).To(gomega.Equal(1))
	log1 = log.V(1)
	log1.Error(err, "Test-1")
	g.Expect(len(log1.(*Logger).Real.(*fake).entry)).To(gomega.Equal(0))
	// level-4
	Settings.Level = Settings.DebugThreshold
	log = WithName("level-testing")
	log0 = log.V(2)
	g.Expect(log0.(*Logger).Real.(*fake).debug).To(gomega.BeFalse())
	log0.Error(err, "Test-0")
	g.Expect(len(log0.(*Logger).Real.(*fake).entry)).To(gomega.Equal(1))
	log1 = log.V(Settings.DebugThreshold)
	log1.Error(err, "Test-1")
	g.Expect(len(log1.(*Logger).Real.(*fake).entry)).To(gomega.Equal(1))
	log2 = log1.V(Settings.DebugThreshold + 1)
	g.Expect(log2.(*Logger).Real.(*fake).debug).To(gomega.BeTrue())
	log2.Error(err, "Test-2")
	g.Expect(len(log2.(*Logger).Real.(*fake).entry)).To(gomega.Equal(0))

	// Test level preserved.
	log3 := log.V(3).WithName("another").WithValues("A", 1)
	g.Expect(log3.(*Logger).name).To(gomega.Equal("another"))
	g.Expect(log3.(*Logger).level).To(gomega.Equal(log3.(*Logger).level))
}
