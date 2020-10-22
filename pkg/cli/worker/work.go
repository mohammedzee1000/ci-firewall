package worker

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mohammedzee1000/ci-firewall/pkg/cli/genericclioptions"
	"github.com/mohammedzee1000/ci-firewall/pkg/jenkins"
	"github.com/mohammedzee1000/ci-firewall/pkg/messages"
	"github.com/mohammedzee1000/ci-firewall/pkg/node"
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
	envVarsArr      []string
	envVars         map[string]string
	sshNodesFile    string
	cimsgenv        string
	cimsg           *messages.RemoteBuildRequestMessage
}

func NewWorkOptions() *WorkOptions {
	return &WorkOptions{
		envVars: make(map[string]string),
	}
}

func (wo *WorkOptions) envVarsArrToEnvVars() error {
	for _, item := range wo.envVarsArr {
		res := strings.Split(item, "=")
		if len(res) != 2 {
			return fmt.Errorf("unable to split envvar, is it in the form FOO=BAR?")
		}
		wo.envVars[res[0]] = res[1]
	}
	return nil
}

func (wo *WorkOptions) Complete(name string, cmd *cobra.Command, args []string) error {
	var err error
	cimsgdata := os.Getenv(wo.cimsgenv)
	if cimsgdata == "" {
		return fmt.Errorf("the env content seems empty, did you provide the right value?")
	}
	wo.cimsg = messages.NewRemoteBuildRequestMessage("", "", "", "", "", "")
	err = json.Unmarshal([]byte(cimsgdata), wo.cimsg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CI message %w", err)
	}
	if wo.cimsg.SetupScript != "" && wo.cimsg.SetupScript[0] != '.' {
		wo.cimsg.SetupScript = fmt.Sprintf("./%s", wo.cimsg.SetupScript)
	}
	if wo.cimsg.RunScript != "" && wo.cimsg.RunScript[0] != '.' {
		wo.cimsg.RunScript = fmt.Sprintf("./%s", wo.cimsg.RunScript)
	}
	return wo.envVarsArrToEnvVars()
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
	if wo.cimsgenv == "" {
		return fmt.Errorf("please provide env of ci message")
	}
	// if wo.repoURL == "" {
	// 	return fmt.Errorf("provide Repo URL")
	// }
	// if wo.kind == "" {
	// 	return fmt.Errorf("provide Kind")
	// }
	// if wo.target == "" {
	// 	return fmt.Errorf("provide Target")
	// }
	// if wo.runScript == "" {
	// 	return fmt.Errorf("provide Run Script")
	// }
	// if wo.kind != messages.RequestTypePR && wo.kind != messages.RequestTypeBranch && wo.kind != messages.RequestTypeTag {
	// 	return fmt.Errorf("kind must be one of these 3 %s|%s|%s", messages.RequestTypePR, messages.RequestTypeBranch, messages.RequestTypeTag)
	// }
	if wo.sshNodesFile != "" {
		_, err := os.Stat(wo.sshNodesFile)
		if err != nil {
			return fmt.Errorf("error stating sshnodefile %w", err)
		}
	}
	return nil
}

func (wo *WorkOptions) Run() (err error) {
	var nl *node.NodeList
	nl = nil
	if wo.sshNodesFile != "" {
		nl, err = node.NodesFromFile(wo.sshNodesFile)
		if err != nil {
			return fmt.Errorf("unable to get node list %w", err)
		}
	}
	wo.worker = worker.NewWorker(
		wo.amqpURI, wo.jenkinsURL, wo.jenkinsUser, wo.jenkinsPassword, wo.jenkinsProject, wo.cimsgenv, wo.cimsg, wo.envVars, wo.jenkinsBuild, nl,
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
	cmd.Flags().StringVar(&o.amqpURI, "amqpurl", os.Getenv("AMQP_URI"), "the url of amqp server")
	cmd.Flags().StringVar(&o.cimsgenv, "cimsgenv", "CI_MESSAGE", "the env containing the CI message")
	cmd.Flags().StringVar(&o.jenkinsURL, "jenkinsurl", jenkins.GetJenkinsURL(), "the url of jenkins server")
	cmd.Flags().StringVar(&o.jenkinsProject, "jenkinsproject", jenkins.GetJenkinsJob(), "the name of the jenkins project")
	cmd.Flags().StringVar(&o.jenkinsUser, "jenkinsuser", os.Getenv("JENKINS_ROBOT_USER"), "the name of the jenkins robot account")
	cmd.Flags().StringVar(&o.jenkinsPassword, "jenkinspassword", os.Getenv("JENKINS_ROBOT_PASSWORD"), "the password of the robot account user")
	cmd.Flags().IntVar(&o.jenkinsBuild, "jenkinsbuild", jenkins.GetJenkinsBuildNumber(), "the number of jenkins build")
	cmd.Flags().StringVar(&o.sshNodesFile, "sshnodesfile", "", "sshnodesfile is path of json file containing node information. If provided tests will be done by sshing to the nodes see docs")
	cmd.Flags().StringArrayVar(&o.envVarsArr, "env", []string{}, "additional env vars to expose to build and run scripts")
	return cmd
}
