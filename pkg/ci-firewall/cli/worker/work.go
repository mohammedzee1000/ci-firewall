package worker

import (
	"encoding/json"
	"fmt"
	"k8s.io/klog/v2"
	"os"
	"strings"

	"github.com/mohammedzee1000/ci-firewall/pkg/ci-firewall/cli/genericclioptions"
	"github.com/mohammedzee1000/ci-firewall/pkg/jenkins"
	"github.com/mohammedzee1000/ci-firewall/pkg/messages"
	"github.com/mohammedzee1000/ci-firewall/pkg/node"
	"github.com/mohammedzee1000/ci-firewall/pkg/worker"
	"github.com/spf13/cobra"
)

const WorkRecommendedCommandName = "work"

type WorkOptions struct {
	worker           *worker.Worker
	amqpURI          string
	jenkinsURL       string
	jenkinsProject   string
	jenkinsBuild     int
	jenkinsUser      string
	jenkinsPassword  string
	envVarsArr       []string
	envVars          map[string]string
	sshNodesFiles    []string
	cimsgenv         string
	standalone       bool
	streambufferSize int
	mainBranch       string
	cimsg            *messages.RemoteBuildRequestMessage
	final            bool
	tags             []string
	stripAnsiColor   bool
	redact bool
	gitUser string
	gitEmail string
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

	klog.V(5).Infof("work options after complete %#v", wo)
	return nil
}

func (wo *WorkOptions) Complete(name string, cmd *cobra.Command, args []string) error {
	klog.V(5).Infof("work options before complete %#v", wo)
	var err error
	klog.V(2).Infof("parsing ci message")
	cimsgdata := os.Getenv(wo.cimsgenv)
	klog.V(4).Infof("ci message extracted from %s looks like %s", wo.cimsgenv, cimsgdata)
	if cimsgdata == "" {
		return fmt.Errorf("the env content seems empty, did you provide the right value?")
	}
	wo.cimsg = messages.NewRemoteBuildRequestMessage("", "", "", "", "", "", "", "", "")
	err = json.Unmarshal([]byte(cimsgdata), wo.cimsg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CI message %w", err)
	}
	klog.V(4).Infof("Marshalled ci message: %#v", wo.cimsg)
	if wo.cimsg.SetupScript != "" && wo.cimsg.SetupScript[0] != '.' {
		wo.cimsg.SetupScript = fmt.Sprintf("./%s", wo.cimsg.SetupScript)
	}
	if wo.cimsg.RunScript != "" && wo.cimsg.RunScript[0] != '.' {
		wo.cimsg.RunScript = fmt.Sprintf("./%s", wo.cimsg.RunScript)
	}
	if wo.standalone {
		wo.amqpURI = ""
	}
	return wo.envVarsArrToEnvVars()
}

func (wo *WorkOptions) Validate() (err error) {
	if !wo.standalone && wo.amqpURI == "" {
		return fmt.Errorf("please provide AMQP URI")
	}
	if wo.streambufferSize <= 5 {
		return fmt.Errorf("stream buffer size should be greater than 5")
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
	if wo.cimsg.RepoURL == "" {
		return fmt.Errorf("CI message missing Repo URL")
	}
	if wo.cimsg.Kind == "" {
		return fmt.Errorf("CI message missing Kind")
	}
	if wo.cimsg.Target == "" {
		return fmt.Errorf("CI message missing Target")
	}
	if wo.cimsg.RunScript == "" {
		return fmt.Errorf("CI message missing Run Script")
	}
	if wo.cimsg.Kind != messages.RequestTypePR && wo.cimsg.Kind != messages.RequestTypeBranch && wo.cimsg.Kind != messages.RequestTypeTag {
		return fmt.Errorf("kind must be one of these 3 %s|%s|%s", messages.RequestTypePR, messages.RequestTypeBranch, messages.RequestTypeTag)
	}
	if wo.cimsg.JenkinsProject != wo.jenkinsProject {
		return fmt.Errorf("jenkins project in ci message does not match name of current project, want %s, got %s", wo.jenkinsProject, wo.cimsg.JenkinsProject)
	}
	if len(wo.sshNodesFiles) > 0 {
		for _, f := range wo.sshNodesFiles {
			_, err := os.Stat(f)
			if err != nil {
				return fmt.Errorf("error stating sshnodefile %w", err)
			}
		}
	}
	if (wo.gitUser != "" && wo.gitEmail == "") || (wo.gitEmail != "" && wo.gitUser == "") {
		return fmt.Errorf("both git user and git email must be provided together or neither of them")
	}
	return nil
}

func (wo *WorkOptions) Run() (err error) {
	var nl *node.NodeList
	nl = nil
	if len(wo.sshNodesFiles) > 0 {
		klog.V(2).Infof("parsing ssh node files")
		nl, err = node.NodesFromFiles(wo.sshNodesFiles)
		if err != nil {
			return fmt.Errorf("unable to get node list %w", err)
		}
		klog.V(4).Infof("parsed node list looks like %#v", nl)
	}
	klog.V(2).Infof("initializing worker")
	wo.worker = worker.NewWorker(
		wo.amqpURI, wo.jenkinsURL, wo.jenkinsUser, wo.jenkinsPassword, wo.jenkinsProject, wo.cimsgenv,
		wo.cimsg, wo.envVars, wo.jenkinsBuild, wo.streambufferSize, nl, wo.final, wo.tags, wo.stripAnsiColor,
		true, wo.gitUser, wo.gitEmail,
	)
	success, err := wo.worker.Run()
	if err != nil {
		return fmt.Errorf("failed to run worker %w", err)
	}
	err = wo.worker.Shutdown()
	if err != nil {
		return fmt.Errorf("failed to shutdown worker %w", err)
	}
	if !success {
		return fmt.Errorf("tests failed, see logs above ^")
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
	cmd.Flags().StringArrayVar(&o.sshNodesFiles, "sshnodesfile", []string{}, "sshnodesfile is path of json file containing node information. If provided tests will be done by sshing to the nodes see docs")
	cmd.Flags().StringArrayVar(&o.envVarsArr, "env", []string{}, "additional env vars to expose to build and run scripts")
	cmd.Flags().BoolVar(&o.standalone, "standalone", false, "is this worker standalone, ie no replyback with message queue")
	cmd.Flags().IntVar(&o.streambufferSize, "streambuffersize", 10, "the size of stream buffer, default to 10.")
	cmd.Flags().BoolVar(&o.final, "final", true, "if true, then final message is sent to requestor (basically telling it there is no more testing left. Allows for pipeline usecases). Default is true")
	cmd.Flags().StringArrayVar(&o.tags, "tag", []string{}, "tags to print in the logs. Note ssh node name is automatically printed")
	cmd.Flags().BoolVar(&o.redact, "redact", true, "if true, then injected envs and ip addresses are redacted from logs sent over queue. Default is true")
	cmd.Flags().StringVar(&o.gitUser, "gituser", "", "The git user you want to configure for repo")
	cmd.Flags().StringVar(&o.gitEmail, "gitemail", "", "The email of git user you want to configure on repo")
	genericclioptions.AddStripANSIColorFlag(cmd, &o.stripAnsiColor)
	return cmd
}
