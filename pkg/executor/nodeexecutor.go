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
	nd        *node.Node
	workdir   string
	session   *ssh.Session
	cmdArgs   []string
	client    *ssh.Client
	envString string
	exitCode  int
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
				ssh.Password(nd.SSHPassword),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	} else {
		return nil, fmt.Errorf("node should either have sshkey or password")
	}
	var addr string
	if nd.Port == "" {
		addr = fmt.Sprintf("%s:22", nd.Address)
	} else {
		addr = fmt.Sprintf("%s:%s", nd.Address, nd.Port)
	}
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to ssh host %w", err)
	}
	session, err := client.NewSession()
	//session.RequestPty("", 100, 100)
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

func (ne *NodeSSHExecutor) BufferedReader() (*bufio.Reader, error) {
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
	cmdstr := strings.Join(ne.cmdArgs, " ")
	if ne.workdir != "" {
		cmdstr = fmt.Sprintf("cd %s && %s", ne.workdir, cmdstr)
	}
	//appendenvstring
	cmdstr = fmt.Sprintf("%s%s", ne.envString, cmdstr)
	err := ne.session.Start(cmdstr)
	if err != nil {
		ne.exitCode = 1
	}
	return err
}

func (ne *NodeSSHExecutor) Wait() error {
	err := ne.session.Wait()
	if err != nil {
		if err, ok := err.(*ssh.ExitError); ok {
			ne.exitCode = err.ExitStatus()
		} else {
			return fmt.Errorf("failed to wait ssh command: %w", err)
		}
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

func (ne *NodeSSHExecutor) SetEnvs(envVars map[string]string, identity string) error {
	envVars[node.NodeBaseOS] = ne.nd.BaseOS
	envVars[node.NodeArch] = ne.nd.Arch
	envVars["SCRIPT_IDENTITY"] = identity
	for k, v := range envVars {
		ne.envString = fmt.Sprintf("%sexport %s=\"%s\"; ", ne.envString, k, v)
	}
	return nil
}

func (ne *NodeSSHExecutor) Session() *ssh.Session {
	return ne.session
}

func (ne *NodeSSHExecutor) CombinedOutput() ([]byte, error) {
	cmdstr := strings.Join(ne.cmdArgs, " ")
	if ne.workdir != "" {
		cmdstr = fmt.Sprintf("cd %s && %s", ne.workdir, cmdstr)
	}

	cmdstr = fmt.Sprintf("%s%s", ne.envString, cmdstr)
	fmt.Println("running ", cmdstr)
	return ne.session.CombinedOutput(cmdstr)
}
