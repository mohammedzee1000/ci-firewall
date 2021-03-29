package main

import (
	"flag"
	"github.com/mohammedzee1000/ci-firewall/pkg/ci-firewall/cli"
	"github.com/mohammedzee1000/ci-firewall/pkg/ci-firewall/cli/genericclioptions"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

func visitSubCommands(cmd *cobra.Command)  {
	if cmd.HasSubCommands() {
		for _, it := range cmd.Commands() {
			visitSubCommands(it)
		}
	}
	cmd.Flags().AddGoFlagSet(flag.CommandLine)
}

func main() {
	root := cli.NewCmdCIFirewall(cli.RecommendedCommandName, cli.RecommendedCommandName)
	klog.InitFlags(nil)
	visitSubCommands(root)
	flag.Parse()
	genericclioptions.LogErrorAndExit(root.Execute(), "")
}
