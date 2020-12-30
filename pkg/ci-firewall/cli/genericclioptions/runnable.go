package genericclioptions

import (
	"log"

	"github.com/spf13/cobra"
)

type Runnable interface {
	Complete(name string, cmd *cobra.Command, args []string) error
	Validate() error
	Run() error
}

func LogErrorAndExit(err error, ctx string) {
	if err != nil {
		log.Fatalf("%s: %s", ctx, err)
	}
}

func GenericRun(o Runnable, cmd *cobra.Command, args []string) {
	// Run completion, validation and run.
	LogErrorAndExit(o.Complete(cmd.Name(), cmd, args), "")
	LogErrorAndExit(o.Validate(), "")
	LogErrorAndExit(o.Run(), "")
}

func AddStripANSIColorFlag(cmd *cobra.Command, v *bool) {
	cmd.Flags().BoolVar(v, "stripansicolor", false, "If true, then ANSI color will be stripped before printing the logs")
}
