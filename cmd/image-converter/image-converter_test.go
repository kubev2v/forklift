package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Image converter", func() {

	Describe("parseQemuimgProgress function", func() {
		It("correctly parses valid progress output", func() {
			line := "(10.00/100%)"
			progress, err := parseQemuimgProgress(line)
			Expect(err).NotTo(HaveOccurred())
			Expect(progress).To(Equal(10.00))
		})

		It("returns an error for invalid progress output", func() {
			line := "Invalid output"
			progress, err := parseQemuimgProgress(line)
			Expect(err).ToNot(HaveOccurred())
			Expect(progress).To(Equal(float64(0)))
		})
	})
})
