package executor

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/mohammedzee1000/ci-firewall/pkg/node"
	"golang.org/x/crypto/ssh"
)

type NodeExecutorWithWorkdir struct {
	nd       *node.Node
	workdir  string
	session  *ssh.Session
	cmdArgs  []string
	exitCode int
}

func NewNodeExecutorWithWorkdir(nd *node.Node, workdir string, cmdArgs []string) (*NodeExecutorWithWorkdir, error) {
	return &NodeExecutorWithWorkdir{
		nd:       nd,
		workdir:  workdir,
		cmdArgs:  cmdArgs,
		exitCode: 0,
	}, nil
}

func (ne *NodeExecutorWithWorkdir) StdoutPipe() (io.ReadCloser, error) {
	r, err := ne.session.StdoutPipe()
	if err != nil {
		return nil, err
	}
	rcl := ioutil.NopCloser(r)
	return rcl, nil
}

func (ne *NodeExecutorWithWorkdir) ShortStderrToStdOut() {
	ne.session.Stderr = ne.session.Stdout
}

func (ne *NodeExecutorWithWorkdir) Start() error {
	cmdstr := strings.Join(ne.cmdArgs, " ")
	if ne.workdir != "" {
		cmdstr = fmt.Sprintf("cd %s && %s", ne.workdir, cmdstr)
	}
	err := ne.session.Start(cmdstr)
	if err != nil {
		ne.exitCode = 1
	}
	return err
}

func (ne *NodeExecutorWithWorkdir) Wait() error {
	err := ne.session.Wait()
	if err != nil {
		ne.exitCode = 1
	}
	return err
}

func (ne *NodeExecutorWithWorkdir) ExitCode() int {
	return ne.exitCode
}

func (ne *NodeExecutorWithWorkdir) Close() error {
	return ne.session.Close()
}

func (ne *NodeExecutorWithWorkdir) SetEnvs(envVars map[string]string) error {
	for k, v := range envVars {
		ne.session.Setenv(k, v)
	}
	return nil
}
