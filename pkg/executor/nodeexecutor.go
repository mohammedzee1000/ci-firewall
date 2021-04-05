package executor

import (
	"bufio"
	"fmt"
	"io"
	"k8s.io/klog/v2"
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
	tags          []string
}

func NewNodeSSHExecutor(nd *node.Node) (*NodeSSHExecutor, error) {
	klog.V(4).Infof("initializing node ssh executor for node %#v", nd)
	var cfg *ssh.ClientConfig
	if nd.SSHKey != "" {
		klog.V(2).Infof("parsing ssh private key")
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
		tags:     []string{fmt.Sprintf("ssh:%s", nd.Name)},
	}, nil
}

func (ne *NodeSSHExecutor) GetName() string {
	return fmt.Sprintf("ssh:%s", ne.nd.Name)
}

func (ne *NodeSSHExecutor) initClient() error {
	var addr string
	var err error
	if ne.nd.Port < 1 {
		addr = fmt.Sprintf("%s:22", ne.nd.Address)
	} else {
		addr = fmt.Sprintf("%s:%d", ne.nd.Address, ne.nd.Port)
	}
	klog.V(4).Infof("establishing ssh connection to node %#v using client config %#v", ne.nd, ne.cfg)
	ne.client, err = ssh.Dial("tcp", addr, ne.cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize ssh client %w", err)
	}
	return nil
}

func (ne *NodeSSHExecutor) InitCommand(workdir string, cmd []string, envVars map[string]string, tags []string) (*bufio.Reader, error) {
	var err error
	klog.V(2).Infof("initializing command")
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
	ne.tags = append([]string{fmt.Sprintf("ssh:%s", ne.nd.Name)}, tags...)
	ne.tags = append(ne.tags, ne.nd.Tags...)
	//appendenvstring
	ne.commandString = fmt.Sprintf("%s%s", envString, ne.commandString)
	klog.V(4).Infof("full ssh command %s", ne.commandString)
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

func (ne *NodeSSHExecutor) GetTags() []string {
	return ne.tags
}

func (ne *NodeSSHExecutor) Start() error {
	if ne.client == nil || ne.session == nil {
		return fmt.Errorf("did you run InitCommand first")
	}
	klog.V(2).Infof("Executing ssh command")
	err := ne.session.Start(ne.commandString)
	if err != nil {
		return fmt.Errorf("failed to start command %w", err)
	}
	return nil
}

func (ne *NodeSSHExecutor) Wait() (bool, error) {
	klog.V(2).Infof("waiting for command execution to complete")
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
	klog.V(2).Infof("closing ssh connection")
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
