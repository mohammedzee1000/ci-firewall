package sendq

import (
	"github.com/mohammedzee1000/ci-firewall/pkg/util"
	"github.com/spf13/cobra"
)

const SendqRecommendedCommandName = "sendq"

func newSendQCommand(name, fullname string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "sendq",
		Long:  "sendq",
	}
	cmd.AddCommand(
		NewCmdSendQueueCreate(SendQueueCreateRecommendedCommandName, util.GetFullName(fullname, SendQueueCreateRecommendedCommandName)),
	)
	return cmd
}

func NewSendQCommand(name, fullname string) *cobra.Command {
	return newSendQCommand(name, fullname)
}
