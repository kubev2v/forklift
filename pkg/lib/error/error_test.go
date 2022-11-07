package error

import (
	"errors"
	"github.com/onsi/gomega"
	errors2 "github.com/pkg/errors"
	"testing"
)

func TestError(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	err := errors.New("failed")
	le := Wrap(err).(*Error)
	g.Expect(le).NotTo(gomega.BeNil())
	g.Expect(le.wrapped).To(gomega.Equal(err))
	g.Expect(len(le.stack)).To(gomega.Equal(4))
	g.Expect(le.Error()).To(gomega.Equal(err.Error()))
	g.Expect(le.Context()).To(gomega.BeNil())

	le2 := Wrap(err).(*Error)
	g.Expect(le2).NotTo(gomega.BeNil())
	g.Expect(le2.wrapped).To(gomega.Equal(err))
	g.Expect(len(le2.stack)).To(gomega.Equal(4))
	g.Expect(le2.Error()).To(gomega.Equal(err.Error()))

	wrapped := errors2.Wrap(err, "help")
	le3 := Wrap(wrapped).(*Error)
	g.Expect(le3).NotTo(gomega.BeNil())
	g.Expect(le3.wrapped).To(gomega.Equal(wrapped))
	g.Expect(le3.wrapped).To(gomega.Equal(wrapped))
	g.Expect(len(le3.stack)).To(gomega.Equal(4))
	g.Expect(errors.Unwrap(le3)).To(gomega.Equal(err))
	g.Expect(len(le3.Context())).To(gomega.Equal(0))
	g.Expect(le3.Error()).To(gomega.Equal("help: failed"))

	le4 := Wrap(
		le3, "Failed to create user.",
		"name", "larry",
		"age", 10)
	g.Expect(le4.(*Error).Error()).To(
		gomega.Equal("Failed to create user. caused by: 'help: failed'"))
	g.Expect(le4.(*Error).Context()).ToNot(gomega.BeNil())
	g.Expect(len(le4.(*Error).Context())).To(gomega.Equal(4))

	le5 := Wrap(
		le4, "Web POST failed.",
		"a", "A",
		"b", "B")
	g.Expect(le5.(*Error).Error()).To(
		gomega.Equal("Web POST failed. caused by: 'Failed to create user.' caused by: 'help: failed'"))
	g.Expect(le5.(*Error).Context()).ToNot(gomega.BeNil())
	g.Expect(len(le5.(*Error).Context())).To(gomega.Equal(8))

	println(le.Stack())
}

func TestNew(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	err := New(
		"create user failed",
		"name", "Elmer",
		"age", 10)
	le := err.(*Error)
	g.Expect(le).NotTo(gomega.BeNil())
	g.Expect(len(le.stack)).To(gomega.Equal(5))
	g.Expect(le.Error()).To(gomega.Equal(err.Error()))
	g.Expect(len(le.Context())).To(gomega.Equal(4))
}

func TestUnwrap(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	err := errors.New("failed")
	g.Expect(err).To(gomega.Equal(Unwrap(err)))
	g.Expect(Unwrap(nil)).To(gomega.BeNil())
	g.Expect(Unwrap(Wrap(err))).To(gomega.Equal(err))
	g.Expect(Unwrap(errors2.Wrap(err, ""))).To(gomega.Equal(err))
	g.Expect(Unwrap(errors2.Wrap(errors2.Wrap(err, ""), ""))).To(gomega.Equal(err))
}
