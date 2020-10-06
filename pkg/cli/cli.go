package cli

import (
	"github.com/mohammedzee1000/ci-firewall/pkg/cli/requestor"
	"github.com/mohammedzee1000/ci-firewall/pkg/cli/worker"
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
		requestor.NewCmdRequestor(requestor.RequestRecommendedCommandName, util.GetFullName(fullname, requestor.RequestRecommendedCommandName)),
		worker.NewWorkCmd(worker.WorkRecommendedCommandName, util.GetFullName(fullname, worker.WorkRecommendedCommandName)),
	)
	return cmd
}
