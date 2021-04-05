package version

import (
	"fmt"
	"github.com/mohammedzee1000/ci-firewall/pkg/ci-firewall/cli/genericclioptions"
	"github.com/mohammedzee1000/ci-firewall/pkg/version"
	"github.com/spf13/cobra"
)

const VersionRecommendedCommandName = "version"

type VersionOptions struct {
}

func NewVersionOptions() *VersionOptions {
	return &VersionOptions{}
}

func (o *VersionOptions) Complete(name string, cmd *cobra.Command, args []string) error {
	return nil
}

func (o *VersionOptions) Validate() (err error) {
	return nil
}

func (o *VersionOptions) Run() (err error) {
	fmt.Println("ci-firewall " + version.VERSION + "(" + version.GITCOMMIT + ")")
	return nil
}

func NewCmdVersion(name, fullName string) *cobra.Command {
	o := NewVersionOptions()
	// versionCmd represents the version command
	var versionCmd = &cobra.Command{
		Use:   name,
		Short: "see the version",
		Run: func(cmd *cobra.Command, args []string) {
			genericclioptions.GenericRun(o, cmd, args)
		},
	}
	return versionCmd
}
