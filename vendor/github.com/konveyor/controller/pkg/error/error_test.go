package error

import (
	"errors"
	"github.com/onsi/gomega"
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

	le2 := Wrap(err).(*Error)
	g.Expect(le2).NotTo(gomega.BeNil())
	g.Expect(le2.wrapped).To(gomega.Equal(err))
	g.Expect(len(le2.stack)).To(gomega.Equal(4))
	g.Expect(le2.Error()).To(gomega.Equal(err.Error()))

	println(le.Stack())
}
