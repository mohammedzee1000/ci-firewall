package executor

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/mohammedzee1000/ci-firewall/pkg/node"
)

type LocalExecutor struct {
	cmd     *exec.Cmd
	workDir string
	currDir string
}

func NewLocalExecutor() *LocalExecutor {
	return &LocalExecutor{}
}

func (le *LocalExecutor) InitCommand(workdir string, cmd []string, envVars map[string]string) (*bufio.Reader, error) {
	var err error
	le.workDir = workdir
	//create command
	if len(cmd) > 0 {
		if len(cmd) == 1 {
			le.cmd = exec.Command(cmd[0])
		} else {
			le.cmd = exec.Command(cmd[0], cmd[1:]...)
		}
	} else {
		return nil, fmt.Errorf("command array should atleast have 1 value")
	}
	//setup env
	le.cmd.Env = os.Environ()
	envVars[node.NodeBaseOS] = "linux"
	envVars[node.NodeArch] = "amd64"
	for k, v := range envVars {
		le.cmd.Env = append(le.cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	//chdir to workdir
	if le.workDir != "" {
		le.currDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get wd %w", err)
		}
		err = os.Chdir(le.workDir)
		if err != nil {
			return nil, fmt.Errorf("failed to change to workdir %w", err)
		}
	}
	//setup the piping
	stdoutpipe, err := le.cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to pipe stdout %w", err)
	}
	stderrpipe, err := le.cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to pipe stderr %w", err)
	}
	return bufio.NewReader(io.MultiReader(stdoutpipe, stderrpipe)), nil
}

func (le *LocalExecutor) Start() error {
	return le.cmd.Start()
}

func (le *LocalExecutor) Wait() (bool, error) {
	err := le.cmd.Wait()
	if err != nil {
		return false, err
	}
	return le.cmd.ProcessState.Success(), err
}

func (le *LocalExecutor) Close() error {
	if le.currDir != "" {
		return os.Chdir(le.currDir)
	}
	return nil
}
