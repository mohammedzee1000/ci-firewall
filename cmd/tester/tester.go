package main

import (
	"fmt"

	"github.com/mohammedzee1000/ci-firewall/pkg/node"
)

func main() {
	nodes, _ := node.NodeListFromDir("test-nodes")
	for _, n := range nodes {
		fmt.Printf("%#v\n", n)
	}
}
