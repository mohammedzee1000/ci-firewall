package nodefile

import (
	"github.com/mohammedzee1000/ci-firewall/pkg/util"
	"github.com/spf13/cobra"
)

const NodefileRecommendedCommandName = "nodefile"

func newNodeFileCommand(name, fullname string) *cobra.Command {
	cmd := &cobra.Command{
		Use: name,
		Short: "nodefile",
		Long: "nodefile",
	}
	cmd.AddCommand(NewCmdNodeFileAddNode(NodefileAddNodeRecommendedCommandName, util.GetFullName(fullname, NodefileAddNodeRecommendedCommandName)))
	return cmd
}

func NewNodeFileCommand(name, fullname string) *cobra.Command {
	return newNodeFileCommand(name, fullname)
}