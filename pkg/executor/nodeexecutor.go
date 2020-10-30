package executor

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/mohammedzee1000/ci-firewall/pkg/node"
	"golang.org/x/crypto/ssh"
)

type NodeSSHExecutor struct {
	nd            *node.Node
	workdir       string
	session       *ssh.Session
	commandString string
	client        *ssh.Client
	cfg           *ssh.ClientConfig
	exitCode      int
}

func NewNodeSSHExecutor(nd *node.Node) (*NodeSSHExecutor, error) {
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
				ssh.Password(nd.SSHPassword),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	} else {
		return nil, fmt.Errorf("node should either have sshkey or password")
	}
	return &NodeSSHExecutor{
		nd:       nd,
		exitCode: 0,
		cfg:      cfg,
		session:  nil,
		client:   nil,
	}, nil
}

func (ne *NodeSSHExecutor) initClient() error {
	var addr string
	var err error
	if ne.nd.Port == "" {
		addr = fmt.Sprintf("%s:22", ne.nd.Address)
	} else {
		addr = fmt.Sprintf("%s:%s", ne.nd.Address, ne.nd.Port)
	}
	ne.client, err = ssh.Dial("tcp", addr, ne.cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize ssh client %w", err)
	}
	return nil
}

func (ne *NodeSSHExecutor) InitCommand(workdir string, cmd []string, envVars map[string]string) (*bufio.Reader, error) {
	var err error
	//setup env
	var envString string
	envVars[node.NodeBaseOS] = ne.nd.BaseOS
	envVars[node.NodeArch] = ne.nd.Arch
	for k, v := range envVars {
		envString = fmt.Sprintf("%sexport %s=\"%s\"; ", envString, k, v)
	}
	ne.commandString = strings.Join(cmd, " ")
	if workdir != "" {
		ne.commandString = fmt.Sprintf("cd %s && %s", workdir, ne.commandString)
	}
	//appendenvstring
	ne.commandString = fmt.Sprintf("%s%s", envString, ne.commandString)
	//setupclient
	err = ne.initClient()
	if err != nil {
		return nil, err
	}
	//reset exit code
	ne.exitCode = 0
	//setup session
	ne.session, err = ne.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("unable to create session %w", err)
	}
	stdoutpipe, err := ne.session.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to pipe stdout %w", err)
	}
	stderrpipe, err := ne.session.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to pipe stderr %w", err)
	}
	return bufio.NewReader(io.MultiReader(stdoutpipe, stderrpipe)), nil
}

func (ne *NodeSSHExecutor) Start() error {
	if ne.client == nil || ne.session == nil {
		return fmt.Errorf("did you run InitCommand first")
	}
	err := ne.session.Start(ne.commandString)
	if err != nil {
		return fmt.Errorf("failed to start command %w", err)
	}
	return nil
}

func (ne *NodeSSHExecutor) Wait() (bool, error) {
	err := ne.session.Wait()
	if err != nil {
		if err, ok := err.(*ssh.ExitError); ok {
			return false, nil
		} else {
			return false, fmt.Errorf("failed to wait ssh command: %w", err)
		}
	}
	return true, nil
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
