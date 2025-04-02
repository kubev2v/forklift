package utils

import (
	"fmt"
	"io"
	"os/exec"
)

//go:generate mockgen -source=command.go -package=utils -destination=mock_command.go
type CommandExecutor interface {
	Run() error
	Start() error
	Wait() error
	SetStdout(io.Writer)
	SetStderr(io.Writer)
	SetStdin(read io.Reader)
}

// RealCommand wraps exec.Cmd.
type Command struct {
	cmd *exec.Cmd
}

// Run executes the command.
func (r *Command) Run() error {
	return r.cmd.Run()
}

// Start starts the specified command but does not wait for it to complete.
func (r *Command) Start() error {
	return r.cmd.Start()
}

// Wait waits for the command to exit and waits for any copying to
// stdin or copying from stdout or stderr to complete.
func (r *Command) Wait() error {
	return r.cmd.Wait()
}

// SetStdout sets the Stdout field of exec.Cmd.
func (r *Command) SetStdout(w io.Writer) {
	r.cmd.Stdout = w
}

// SetStderr sets the Stderr field of exec.Cmd.
func (r *Command) SetStderr(w io.Writer) {
	r.cmd.Stderr = w
}

// SetStdin sets the Stderr field of exec.Cmd.
func (r *Command) SetStdin(read io.Reader) {
	r.cmd.Stdin = read
}

//go:generate mockgen -source=command.go -package=utils -destination=mock_command.go
type CommandBuilder interface {
	New(cmd string) CommandBuilder
	AddArg(flag string, value string) CommandBuilder
	AddArgs(flag string, values ...string) CommandBuilder
	AddFlag(flag string) CommandBuilder
	AddPositional(value string) CommandBuilder
	AddExtraArgs(values ...string) CommandBuilder
	Build() CommandExecutor
}

type CommandBuilderImpl struct {
	BaseCommand string
	Args        []string
	// Executor
	Executor CommandExecutor
}

func (cb *CommandBuilderImpl) New(cmd string) CommandBuilder {
	cb.BaseCommand = cmd
	cb.Args = []string{}
	return cb
}

func (cb *CommandBuilderImpl) AddArg(flag string, value string) CommandBuilder {
	if value != "" {
		cb.Args = append(cb.Args, flag, value)
	}
	return cb
}

func (cb *CommandBuilderImpl) AddArgs(flag string, values ...string) CommandBuilder {
	for _, value := range values {
		if value != "" {
			cb.Args = append(cb.Args, flag, value)
		}
	}
	return cb
}

func (cb *CommandBuilderImpl) AddFlag(flag string) CommandBuilder {
	cb.Args = append(cb.Args, flag)
	return cb
}

// Add a single parameter that is NOT a flag (e.g., a file path or a command action)
func (cb *CommandBuilderImpl) AddPositional(value string) CommandBuilder {
	if value != "" {
		cb.Args = append(cb.Args, value)
	}
	return cb
}

func (cb *CommandBuilderImpl) AddExtraArgs(values ...string) CommandBuilder {
	cb.Args = append(cb.Args, values...)
	return cb
}

func (cb *CommandBuilderImpl) Build() CommandExecutor {
	fmt.Print("Building command:", cb.BaseCommand, cb.Args)
	return &Command{cmd: exec.Command(cb.BaseCommand, cb.Args...)}
}
