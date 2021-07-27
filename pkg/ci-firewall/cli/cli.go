package cli

import (
	"github.com/mohammedzee1000/ci-firewall/pkg/ci-firewall/cli/requestor"
	"github.com/mohammedzee1000/ci-firewall/pkg/ci-firewall/cli/toolkit"
	"github.com/mohammedzee1000/ci-firewall/pkg/ci-firewall/cli/version"
	"github.com/mohammedzee1000/ci-firewall/pkg/ci-firewall/cli/worker"
	"github.com/mohammedzee1000/ci-firewall/pkg/util"
	"github.com/spf13/cobra"
)

const RecommendedCommandName = "ci-firewall"

func NewCmdCIFirewall(name, fullname string) *cobra.Command {
	rootCmd := ciFirewallCmd(name, fullname)
	return rootCmd
}

func ciFirewallCmd(name, fullname string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "ci-firewall",
		Long:  "ci-firewall",
	}
	cmd.AddCommand(
		requestor.NewCmdRequester(requestor.RequestRecommendedCommandName, util.GetFullName(fullname, requestor.RequestRecommendedCommandName)),
		worker.NewWorkCmd(worker.WorkRecommendedCommandName, util.GetFullName(fullname, worker.WorkRecommendedCommandName)),
		version.NewCmdVersion(version.VersionRecommendedCommandName, util.GetFullName(fullname, version.VersionRecommendedCommandName)),
		toolkit.NewToolkitCommand(toolkit.ToolKitRecommendedCommandName, util.GetFullName(fullname, toolkit.ToolKitRecommendedCommandName)),
	)
	return cmd
}
