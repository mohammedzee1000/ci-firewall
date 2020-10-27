package main

import (
	"github.com/mohammedzee1000/ci-firewall/pkg/ci-firewall/cli"
	"github.com/mohammedzee1000/ci-firewall/pkg/ci-firewall/cli/genericclioptions"
)

func main() {
	root := cli.NewCmdCIFirewall(cli.RecommendedCommandName, cli.RecommendedCommandName)
	genericclioptions.LogErrorAndExit(root.Execute(), "")
}
