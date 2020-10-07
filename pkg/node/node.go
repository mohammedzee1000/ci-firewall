package node

import "fmt"

type Node struct {
	OS          string
	User        string
	NIP         string
	BaseOS      string
	Arch        string
	SSHPAssword string
	sshKey      string
}

func (nd *Node) NodeSSHCommand(cmdArgs string) []string {
	var newCmdArgs []string
	//TODO impl this
	newCmdArgs = append(newCmdArgs, fmt.Sprintf("\"%s\"", cmdArgs))
	return newCmdArgs
}

func (nd *Node) GenerateEnvFile(envfile string, origvalues map[string]string) error {
	//TODO impl
	return nil
}

func (nd *Node) SCPToWorkDirCommand(what string, workdir string) []string {
	var cmdArgs []string
	//TODO implement this
	return cmdArgs
}

type NodeList []Node

func NodeListFromDir(nodedir string) (NodeList, error) {
	var nl NodeList
	//TODO implement this
	return nl, nil
}
