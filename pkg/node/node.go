package node

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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

func newNode() *Node {
	return &Node{}
}

func newNodeList() *NodeList {
	return &NodeList{}
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
