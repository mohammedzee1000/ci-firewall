package worker

import (
	"fmt"

	"github.com/mohammedzee1000/ci-firewall/pkg/cli/genericclioptions"
	"github.com/mohammedzee1000/ci-firewall/pkg/messages"
	"github.com/mohammedzee1000/ci-firewall/pkg/worker"
	"github.com/spf13/cobra"
)

const WorkRecommendedCommandName = "work"

type WorkOptions struct {
	worker          *worker.Worker
	amqpURI         string
	jenkinsURL      string
	jenkinsProject  string
	jenkinsBuild    int
	jenkinsUser     string
	jenkinsPassword string
	repoURL         string
	kind            string
	target          string
	runScript       string
	recieveQName    string
	multiNode       bool
}

func NewWorkOptions() *WorkOptions {
	return &WorkOptions{}
}

func (wo *WorkOptions) Complete(name string, cmd *cobra.Command, args []string) error {
	if wo.recieveQName == "" {
		wo.recieveQName = fmt.Sprintf("rcv_%s_%s", wo.jenkinsProject, wo.target)
	}
	if wo.kind == "" {
		wo.kind = messages.RequestTypePR
	}
	return nil
}

func (wo *WorkOptions) Validate() (err error) {
	if wo.amqpURI == "" {
		return fmt.Errorf("provide AMQP URI")
	}
	if wo.jenkinsProject == "" {
		return fmt.Errorf("provide Jenkins Project")
	}
	if wo.jenkinsURL == "" {
		return fmt.Errorf("provide Jenkins URL")
	}
	if wo.jenkinsUser == "" {
		return fmt.Errorf("provide Jenkins user")
	}
	if wo.jenkinsPassword == "" {
		return fmt.Errorf("provide Jenkins user")
	}
	if wo.repoURL == "" {
		return fmt.Errorf("provide Repo URL")
	}
	if wo.kind == "" {
		return fmt.Errorf("provide Kind")
	}
	if wo.target == "" {
		return fmt.Errorf("provide Target")
	}
	if wo.runScript == "" {
		return fmt.Errorf("provide Run Script")
	}
	if wo.kind != messages.RequestTypePR && wo.kind != messages.RequestTypeBranch && wo.kind != messages.RequestTypeTag {
		return fmt.Errorf("kind must be one of these 3 %s|%s|%s", messages.RequestTypePR, messages.RequestTypeBranch, messages.RequestTypeTag)
	}
	return nil
}

func (wo *WorkOptions) Run() (err error) {
	wo.worker = worker.NewWorker(
		wo.amqpURI, wo.jenkinsURL, wo.jenkinsUser, wo.jenkinsPassword, wo.jenkinsProject, wo.kind, wo.repoURL, wo.target, wo.runScript, wo.recieveQName, wo.jenkinsBuild, wo.multiNode,
	)
	err = wo.worker.Run()
	if err != nil {
		return fmt.Errorf("failed to run worker %w", err)
	}
	err = wo.worker.Shutdown()
	if err != nil {
		return fmt.Errorf("failed to shutdown worker %w", err)
	}
	return nil
}

func NewWorkCmd(name, fullname string) *cobra.Command {
	o := NewWorkOptions()
	cmd := &cobra.Command{
		Use:   name,
		Short: "work on a build",
		Run: func(cmd *cobra.Command, args []string) {
			genericclioptions.GenericRun(o, cmd, args)
		},
	}
	cmd.Flags().StringVar(&o.amqpURI, "amqpurl", "", "the url of amqp server")
	cmd.Flags().StringVar(&o.recieveQName, "recievequeue", "", "the name of the recieve queue")
	cmd.Flags().StringVar(&o.jenkinsURL, "jenkinsurl", "", "the url of jenkins server")
	cmd.Flags().StringVar(&o.jenkinsProject, "jenkinsproject", "", "the name of the jenkins project")
	cmd.Flags().StringVar(&o.jenkinsUser, "jenkinsuser", "", "the name of the jenkins robot account")
	cmd.Flags().StringVar(&o.jenkinsPassword, "jenkinspassword", "", "the password of the robot account user")
	cmd.Flags().IntVar(&o.jenkinsBuild, "jenkinsbuild", -1, "the number of jenkins build")
	cmd.Flags().StringVar(&o.repoURL, "repourl", "", "the url of the repo to clone on jenkins")
	cmd.Flags().StringVar(&o.kind, "kind", "", "the kind of build you want to do")
	cmd.Flags().StringVar(&o.target, "target", "", "the target is based on kind. Can be pr no or branch name or tag name")
	cmd.Flags().StringVar(&o.runScript, "run", "", "the path of the script to run on jenkins, relative to repo root")
	cmd.Flags().BoolVar(&o.multiNode, "multinode", false, "multinode is used to run tests on different nodes, see docs")
	return cmd
}
