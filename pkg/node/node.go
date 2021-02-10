package node

import (
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"os"
)

const (
	NodeBaseOS = "BASE_OS"
	NodeArch   = "ARCH"
)

type Node struct {
	Name        string   `json:"name"`
	User        string   `json:"user"`
	Address     string   `json:"address"`
	Port        int      `json:"port,omitempty"`
	BaseOS      string   `json:"baseos"`
	Arch        string   `json:"arch"`
	SSHPassword string   `json:"password"`
	SSHKey      string   `json:"privatekey"`
	Tags        []string `json:"tags"`
}

type NodeList struct {
	Nodes []Node `json:"nodes"`
}

func newEmptyNode() *Node {
	return &Node{}
}

func newNodeList() *NodeList {
	return &NodeList{}
}

func newNode(name, user, address, baseos, arch, sshpasswd, privatekey string, tags []string, port int) *Node {
	return &Node{
		Name:        name,
		User:        user,
		Address:     address,
		Port:        port,
		BaseOS:      baseos,
		Arch:        arch,
		SSHPassword: sshpasswd,
		SSHKey:      privatekey,
		Tags:        tags,
	}
}

func NodesFromJson(nodejson []byte) (*NodeList, error) {
	nl := newNodeList()
	err := json.Unmarshal(nodejson, nl)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal nodeinfo json %w", err)
	}
	return nl, nil
}

func NodesFromFile(filepath string) (*NodeList, error) {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read nodeinfo from file %w", err)
	}
	return NodesFromJson(data)
}

func NodesFromFiles(files []string) (*NodeList, error) {
	nl := newNodeList()
	for _, f := range files {
		cnl, err := NodesFromFile(f)
		if err != nil {
			return nil, err
		}
		nl.Nodes = append(nl.Nodes, cnl.Nodes...)
	}
	return nl, nil
}

func AddNodeToFile(filepath, name, user, address, baseos, arch, sshpasswd, privatekeyfile string, tags []string, port int) error {
	var nl *NodeList
	var privatekey string
	if privatekeyfile != "" {
		d, err := ioutil.ReadFile(privatekeyfile)
		if err != nil {
			return fmt.Errorf("failed to read private key from provided file path: %w", err)
		}
		_, err = ssh.ParsePrivateKey(d)
		if err != nil {
			return fmt.Errorf("failed to parse ssh key after reading")
		}
		privatekey = string(d)
		_, err = ssh.ParsePrivateKey(d)
		if err != nil {
			return fmt.Errorf("failed to parse ssh key after converting to string")
		}
	}
	nd := newNode(name, user, address, baseos, arch, sshpasswd, privatekey, tags, port)
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		nl = newNodeList()
	} else {
		nl, err = NodesFromFile(filepath)
		if err != nil {
			return fmt.Errorf("failed to read nodelist from existing file: %w", err)
		}
	}
	nl.Nodes = append(nl.Nodes, *nd)
	nlj, err := json.Marshal(nl)
	if err != nil {
		return fmt.Errorf("failed to unmarshall node list: %w", err)
	}
	return ioutil.WriteFile(filepath, []byte(nlj), os.ModePerm)
}
