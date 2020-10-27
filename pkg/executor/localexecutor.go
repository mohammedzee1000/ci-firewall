package executor

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"

	"github.com/mohammedzee1000/ci-firewall/pkg/node"
)

type LocalExecutor struct {
	cmd *exec.Cmd
}

func NewLocalExecutor(cmdArgs []string) *LocalExecutor {
	return &LocalExecutor{
		cmd: exec.Command(cmdArgs[0], cmdArgs[1:]...),
	}
}

func (le *LocalExecutor) BufferedReader() (*bufio.Reader, error) {
	le.cmd.Stderr = le.cmd.Stdout
	stdoutpipe, err := le.cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe %w", err)
	}
	return bufio.NewReader(stdoutpipe), nil
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
	envVars[node.NodeBaseOS] = "linux"
	envVars[node.NodeArch] = "amd64"
	for k, v := range envVars {
		le.cmd.Env = append(le.cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	return nil
}

func (le *LocalExecutor) Close() error {
	return nil
}
