package main

import (
	"os"

	"github.com/yaacov/kubectl-mtv/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
