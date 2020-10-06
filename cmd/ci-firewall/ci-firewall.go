package main

import (
	"github.com/mohammedzee1000/ci-firewall/pkg/cli"
	"github.com/mohammedzee1000/ci-firewall/pkg/cli/genericclioptions"
)

func main() {
	root := cli.NewCmdCIFirewall(cli.RecommendedCommandName, cli.RecommendedCommandName)
	genericclioptions.LogErrorAndExit(root.Execute(), "")
}
