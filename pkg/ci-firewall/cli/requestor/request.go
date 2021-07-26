package requestor

import (
	"fmt"
	"os"
	"time"

	"k8s.io/klog/v2"

	"github.com/mohammedzee1000/ci-firewall/pkg/ci-firewall/cli/genericclioptions"
	"github.com/mohammedzee1000/ci-firewall/pkg/jenkins"
	"github.com/mohammedzee1000/ci-firewall/pkg/messages"
	"github.com/mohammedzee1000/ci-firewall/pkg/requestor"
	"github.com/spf13/cobra"
)

const RequestRecommendedCommandName = "request"

type RequestOptions struct {
	requester        *requestor.Requester
	amqpURI          string
	sendQName        string
	sendExchangeName string
	sendTopic        string
	repoURL          string
	kind             messages.RequestType
	kindVal          string
	target           string
	runScript        string
	setupScript      string
	rcvIdent         string
	jenkinsProject   string
	runScriptURL     string
	mainBranch       string
	timeout          time.Duration
	lazy             bool
}

func NewRequestOptions() *RequestOptions {
	return &RequestOptions{}
}

func (ro *RequestOptions) Complete(name string, cmd *cobra.Command, args []string) error {
	klog.V(5).Infof("request options before complete %#v", ro)
	if ro.kindVal == "" {
		ro.kind = messages.RequestTypePR
	} else {
		ro.kind = messages.RequestType(ro.kindVal)
	}
	if ro.rcvIdent == "" {
		ro.rcvIdent = fmt.Sprintf("amqp.ci.rcv.%s.%s.%s", ro.jenkinsProject, ro.kind, ro.target)
	}
	if ro.lazy {
		ro.rcvIdent = fmt.Sprintf("%s.lazy", ro.rcvIdent)
	}
	klog.V(5).Infof("request options after complete %#v", ro)
	return nil
}

func (ro *RequestOptions) Validate() (err error) {
	if ro.amqpURI == "" {
		return fmt.Errorf("provide AMQP URI")
	}
	if ro.sendExchangeName == "" {
		return fmt.Errorf("please provide send exchange name")
	}
	if ro.sendTopic == "" {
		return fmt.Errorf("please provide send q topic")
	}
	if ro.repoURL == "" {
		return fmt.Errorf("provide Repo URL")
	}
	if ro.kind == "" {
		return fmt.Errorf("provide Kind")
	}
	if ro.target == "" {
		return fmt.Errorf("provide Target")
	}
	if ro.runScript == "" {
		return fmt.Errorf("provide Run Script")
	}
	if ro.kind != messages.RequestTypePR && ro.kind != messages.RequestTypeBranch && ro.kind != messages.RequestTypeTag {
		return fmt.Errorf("kind must be one of these 3 %s|%s|%s", messages.RequestTypePR, messages.RequestTypeBranch, messages.RequestTypeTag)
	}
	if ro.kind == messages.RequestTypePR && ro.mainBranch == "" {
		return fmt.Errorf("main branch must be provided if kind is pr")
	}
	return nil
}

func (ro *RequestOptions) Run() (err error) {
	klog.V(2).Infof("initializing requester")
	nro := requestor.NewRequesterOptions{
		AMQPURI:          ro.amqpURI,
		SendQueueName:    ro.sendQName,
		ExchangeName:     ro.sendExchangeName,
		Topic:            ro.sendTopic,
		RepoURL:          ro.repoURL,
		Kind:             ro.kind,
		Target:           ro.target,
		SetupScript:      ro.setupScript,
		RunScript:        ro.runScript,
		ReceiveQueueName: ro.rcvIdent,
		RunScriptURL:     ro.runScriptURL,
		MainBranch:       ro.mainBranch,
		JenkinsProject:   ro.jenkinsProject,
	}
	ro.requester = requestor.NewRequester(&nro)
	klog.V(4).Infof("requester looks like %#v", ro.requester)
	err = ro.requester.Run()
	if err != nil {
		return err
	}
	return nil
}

func NewCmdRequester(name, fullname string) *cobra.Command {
	o := NewRequestOptions()
	cmd := &cobra.Command{
		Use:   name,
		Short: "request a build",
		Run: func(cmd *cobra.Command, args []string) {
			genericclioptions.GenericRun(o, cmd, args)
		},
	}
	cmd.Flags().StringVar(&o.amqpURI, "amqpuri", os.Getenv("AMQP_URI"), "the url of amqp server")
	cmd.Flags().StringVar(&o.jenkinsProject, "jenkinsproject", jenkins.GetJenkinsJob(), "the name of target jenkins project. Required for ident purposes only")
	cmd.Flags().StringVar(&o.sendQName, "sendqueue", "amqp.ci.queue.send", "the name of the send queue")
	cmd.Flags().StringVar(&o.sendExchangeName, "sendexchange", "amqp.ci.exchange.send", "the name of the exchange tp use for send")
	cmd.Flags().StringVar(&o.sendTopic, "sendtopic", "amqp.ci.topic.send", "the name of the send topic")
	cmd.Flags().StringVar(&o.rcvIdent, "rcvident", os.Getenv(messages.RequestParameterRcvQueueName), "the name of the recieve queue")
	cmd.Flags().StringVar(&o.repoURL, "repourl", os.Getenv(messages.RequestParameterRepoURL), "the url of the repo to clone on jenkins")
	cmd.Flags().StringVar(&o.kindVal, "kind", os.Getenv(messages.RequestParameterKind), "the kind of build you want to do")
	cmd.Flags().StringVar(&o.target, "target", os.Getenv(messages.RequestParameterTarget), "the target is based on kind. Can be pr no or branch name or tag name")
	cmd.Flags().StringVar(&o.runScript, "runscript", os.Getenv(messages.RequestParameterRunScript), "the path of the script to run on jenkins, relative to repo root")
	cmd.Flags().StringVar(&o.runScriptURL, "runscripturl", "", "the url of remote run script, if any. Must be providede with --runscript as that is what it will be downloaded as")
	cmd.Flags().StringVar(&o.setupScript, "setupscript", os.Getenv(messages.RequestParameterSetupScript), "the setup script to run")
	cmd.Flags().DurationVar(&o.timeout, "timeout", 15*time.Minute, "timeout duration ")
	cmd.Flags().StringVar(&o.mainBranch, "mainbranch", "main", "the main branch, to be provided if kind is PR")
	cmd.Flags().BoolVar(&o.lazy, "lazy", false, "Use lazy queues. This simply appends lazy to rcv queue name. So configure rabbitmq accordingly. see https://www.rabbitmq.com/lazy-queues.html")
	return cmd
}
