package node

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

type Node struct {
	Name        string
	User        string
	NIP         string
	BaseOS      string
	Arch        string
	SSHPAssword string
	sshKey      ssh.Signer
}

type NodeList []Node

func NodeListFromDir(nodedir string) (NodeList, error) {
	var nl NodeList
	//TODO implement this
	err := filepath.Walk(nodedir, func(path string, info os.FileInfo, err error) error {
		return nil
	})
	if err != nil {
		return nl, fmt.Errorf("unable to find node files %w", err)
	}
	return nl, nil
}
