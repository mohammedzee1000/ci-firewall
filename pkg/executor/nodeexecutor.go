package executor

import (
	"io"
	"io/ioutil"
	"strings"

	"github.com/mohammedzee1000/ci-firewall/pkg/node"
	"golang.org/x/crypto/ssh"
)

type NodeExecutor struct {
	nd       *node.Node
	session  *ssh.Session
	cmdArgs  []string
	exitCode int
}

func NewNodeExecutor(nd *node.Node, cmdArgs []string) (*NodeExecutor, error) {
	return &NodeExecutor{
		nd:       nd,
		cmdArgs:  cmdArgs,
		exitCode: 0,
	}, nil
}

func (ne *NodeExecutor) StdoutPipe() (io.ReadCloser, error) {
	r, err := ne.session.StdoutPipe()
	if err != nil {
		return nil, err
	}
	rcl := ioutil.NopCloser(r)
	return rcl, nil
}

func (ne *NodeExecutor) ShortStderrToStdOut() {
	ne.session.Stderr = ne.session.Stdout
}

func (ne *NodeExecutor) Start() error {
	err := ne.session.Start(strings.Join(ne.cmdArgs, " "))
	if err != nil {
		ne.exitCode = 1
	}
	return err
}

func (ne *NodeExecutor) Wait() error {
	err := ne.session.Wait()
	if err != nil {
		ne.exitCode = 1
	}
	return err
}

func (ne *NodeExecutor) ExitCode() int {
	return ne.exitCode
}

func (ne *NodeExecutor) Close() error {
	return ne.session.Close()
}

func (ne *NodeExecutor) SetEnvs(envVars map[string]string) error {
	for k, v := range envVars {
		ne.session.Setenv(k, v)
	}
	return nil
}
