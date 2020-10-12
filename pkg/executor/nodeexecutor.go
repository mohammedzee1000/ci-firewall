package executor

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/mohammedzee1000/ci-firewall/pkg/node"
	"golang.org/x/crypto/ssh"
)

type NodeSSHExecutor struct {
	nd       *node.Node
	workdir  string
	session  *ssh.Session
	cmdArgs  []string
	client   *ssh.Client
	exitCode int
}

func NewNodeSSHExecutor(nd *node.Node, workdir string, cmdArgs []string) (*NodeSSHExecutor, error) {
	var cfg *ssh.ClientConfig
	if nd.SSHKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(nd.SSHKey))
		if err != nil {
			return nil, fmt.Errorf("unable to parse private key %w", err)
		}
		cfg = &ssh.ClientConfig{
			User: nd.User,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	} else if nd.SSHPassword != "" {
		cfg = &ssh.ClientConfig{
			User: nd.User,
			Auth: []ssh.AuthMethod{
				ssh.Password("yourpassword"),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	} else {
		return nil, fmt.Errorf("node should either have sshkey or password")
	}
	addr := nd.Address
	if len(strings.Split(addr, ":")) < 2 {
		addr = fmt.Sprintf("%s:22", addr)
	}
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to ssh host %w", err)
	}
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("unable to create session %w", err)
	}
	return &NodeSSHExecutor{
		nd:       nd,
		workdir:  workdir,
		cmdArgs:  cmdArgs,
		exitCode: 0,
		client:   client,
		session:  session,
	}, nil
}

func (ne *NodeSSHExecutor) StdoutPipe() (io.ReadCloser, error) {
	r, err := ne.session.StdoutPipe()
	if err != nil {
		return nil, err
	}
	rcl := ioutil.NopCloser(r)
	return rcl, nil
}

func (ne *NodeSSHExecutor) ShortStderrToStdOut() {
	ne.session.Stderr = ne.session.Stdout
}

func (ne *NodeSSHExecutor) Start() error {
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

func (ne *NodeSSHExecutor) Wait() error {
	err := ne.session.Wait()
	if err != nil {
		ne.exitCode = 1
	}
	return err
}

func (ne *NodeSSHExecutor) ExitCode() int {
	return ne.exitCode
}

func (ne *NodeSSHExecutor) Close() error {
	err := ne.session.Close()
	if err != nil {
		return fmt.Errorf("unable to close session %w", err)
	}
	err = ne.client.Close()
	if err != nil {
		return fmt.Errorf("failed to close ssh client %w", err)
	}
	return nil
}

func (ne *NodeSSHExecutor) SetEnvs(envVars map[string]string) error {
	for k, v := range envVars {
		ne.session.Setenv(k, v)
	}
	return nil
}
