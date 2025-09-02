package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPopulatorController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Populator Controller Suite")
}
