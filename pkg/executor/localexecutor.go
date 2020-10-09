package executor

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

type LocalExecutor struct {
	cmd *exec.Cmd
}

func NewLocalExecutor(cmdArgs []string) *LocalExecutor {
	return &LocalExecutor{
		cmd: exec.Command(cmdArgs[0], cmdArgs[1:]...),
	}
}

func (le *LocalExecutor) StdoutPipe() (io.ReadCloser, error) {
	return le.cmd.StdoutPipe()
}

func (le *LocalExecutor) ShortStderrToStdOut() {
	le.cmd.Stderr = le.cmd.Stdout
}

func (le *LocalExecutor) Start() error {
	return le.cmd.Start()
}

func (le *LocalExecutor) Wait() error {
	return le.cmd.Wait()
}

func (le *LocalExecutor) ExitCode() int {
	return le.cmd.ProcessState.ExitCode()
}

func (le *LocalExecutor) SetEnvs(envVars map[string]string) error {
	le.cmd.Env = os.Environ()
	envVars["BASE_OS"] = "linux"
	envVars["ARCH"] = "amd64"
	for k, v := range envVars {
		le.cmd.Env = append(le.cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	return nil
}

func (le *LocalExecutor) Close() error {
	return nil
}
