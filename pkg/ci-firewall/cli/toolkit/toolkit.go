package toolkit

import (
	"github.com/mohammedzee1000/ci-firewall/pkg/ci-firewall/cli/toolkit/nodefile"
	"github.com/mohammedzee1000/ci-firewall/pkg/ci-firewall/cli/toolkit/sendq"
	"github.com/mohammedzee1000/ci-firewall/pkg/util"
	"github.com/spf13/cobra"
)

const ToolKitRecommendedCommandName = "toolkit"

func NewToolkitCommand(name, fullname string) *cobra.Command {
	return newToolKitCommand(name, fullname)
}

func newToolKitCommand(name, fullname string) *cobra.Command {
	cmd := &cobra.Command{
		Use: name,
		Short: "toolkit",
		Long: "toolkit",
	}
	cmd.AddCommand(
		sendq.NewSendQCommand(sendq.SendqRecommendedCommandName, util.GetFullName(fullname, sendq.SendqRecommendedCommandName)),
		nodefile.NewNodeFileCommand(nodefile.NodefileRecommendedCommandName, util.GetFullName(fullname, nodefile.NodefileRecommendedCommandName)),
	)
	return cmd
}