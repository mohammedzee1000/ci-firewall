package worker

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/mohammedzee1000/ci-firewall/pkg/jenkins"
	"github.com/mohammedzee1000/ci-firewall/pkg/messages"
	"github.com/mohammedzee1000/ci-firewall/pkg/node"
	"github.com/mohammedzee1000/ci-firewall/pkg/printstreambuffer"
	"github.com/mohammedzee1000/ci-firewall/pkg/queue"
)

const scriptIdentity = "SCRIPT_IDENTITY"

//Worker works on a request for test build
type Worker struct {
	receiveQueue     *queue.AMQPQueue
	jenkinsProject   string
	jenkinsBuild     int
	jenkinsURL       string
	jenkinsUser      string
	jenkinsPassword  string
	ciMessageEnv     string
	ciMessage        *messages.RemoteBuildRequestMessage
	envVars          map[string]string
	repoDir          string
	sshNodes         *node.NodeList
	psb              *printstreambuffer.PrintStreamBuffer
	final            bool
	tags             []string
	stripANSIColor   bool
	redact           bool
	gitUser          string
	gitEmail         string
	filterFunc       func(params map[string]string) bool
	retryLoopCount   int
	retryLoopBackOff time.Duration
	redactExceptions []string
}

type NewWorkerOptions struct {
	AMQPURI               string
	JenkinsURL            string
	JenkinsUser           string
	JenkinsPassword       string
	JenkinsProject        string
	CIMessageEnv          string
	CIMessage             *messages.RemoteBuildRequestMessage
	EnvVars               map[string]string
	JenkinsBuild          int
	PrintStreamBufferSize int
	SSHNodes              *node.NodeList
	Final                 bool
	Tags                  []string
	StripANSIColor        bool
	Redact                bool
	GitUser               string
	GitEmail              string
	RetryLoopCount        int
	RetryLoopBackOff      time.Duration
	RedactExceptions      []string
}

//NewWorker creates a new worker struct. if standalone is true, then rabbitmq is not used for communication with requestor.
//Instead ciMessage must be provided manually(see readme). AMQPURI is full uri (including username and password) of rabbitmq server.
//jenkinsURL, jenkinsUser, jenkinsPassword, jenkinsProject are info related to jenkins (robot account is used for cancelling
//older builds by matching job parameter cienvmsg). ciMessage is parsed CI message. to provide nessasary info to worker and also match
//and cleanup older jenkins jobs. envVars are envs to be exposed to the setup and run scripts . jenkinsBuild is current jenkins build
//number. psbSize is max buffer size for PrintStreamBuffer and sshNode is a parsed sshnodefile (see readme)
func NewWorker(wo *NewWorkerOptions) *Worker {
	w := &Worker{
		receiveQueue:     nil,
		ciMessage:        wo.CIMessage,
		jenkinsProject:   wo.JenkinsProject,
		jenkinsBuild:     wo.JenkinsBuild,
		jenkinsURL:       wo.JenkinsURL,
		jenkinsUser:      wo.JenkinsUser,
		jenkinsPassword:  wo.JenkinsPassword,
		envVars:          wo.EnvVars,
		repoDir:          "repo",
		sshNodes:         wo.SSHNodes,
		final:            wo.Final,
		tags:             wo.Tags,
		stripANSIColor:   wo.StripANSIColor,
		redact:           wo.Redact,
		redactExceptions: wo.RedactExceptions,
		ciMessageEnv:     wo.CIMessageEnv,
		gitUser:          wo.GitUser,
		gitEmail:         wo.GitEmail,
		retryLoopCount:   wo.RetryLoopCount,
		retryLoopBackOff: wo.RetryLoopBackOff,
	}
	if wo.AMQPURI != "" {
		w.receiveQueue = queue.NewAMQPQueue(wo.AMQPURI, wo.CIMessage.ReceiveQueueName)
	}
	klog.V(2).Infof("setting script identity")
	w.envVars[scriptIdentity] = strings.ToLower(fmt.Sprintf("%s%s%s", wo.JenkinsProject, wo.CIMessage.Kind, wo.CIMessage.Target))
	klog.V(2).Infof("initializing printstreambuffer and print and stream logs")
	w.psb = printstreambuffer.NewPrintStreamBuffer(w.receiveQueue, wo.PrintStreamBufferSize, w.jenkinsBuild, w.jenkinsProject, w.envVars, w.redact, []string{})
	return w
}

func (w *Worker) initFilterFunc() {
	w.filterFunc = func(params map[string]string) bool {
		for k, v := range params {
			if k == w.ciMessageEnv {
				//v is the ciMessage of this job
				klog.V(3).Infof("parsing ci message for build being looked at")
				jcim := messages.NewRemoteBuildRequestMessage("", "", "", "", "", "", "", "", "")
				json.Unmarshal([]byte(v), jcim)
				if jcim.Kind == w.ciMessage.Kind && jcim.ReceiveQueueName == w.ciMessage.ReceiveQueueName && jcim.RepoURL == w.ciMessage.RepoURL && jcim.Target == w.ciMessage.Target && jcim.RunScript == w.ciMessage.RunScript && jcim.SetupScript == w.ciMessage.SetupScript && jcim.RunScriptURL == w.ciMessage.RunScriptURL && jcim.MainBranch == w.ciMessage.MainBranch && jcim.JenkinsProject == w.ciMessage.JenkinsProject {
					return true
				}
			}
		}
		return false
	}
}

func (w *Worker) checkForNewerBuilds() (bool, error) {
	exists, err := jenkins.NewerBuildsExist(w.jenkinsURL, w.jenkinsUser, w.jenkinsPassword, w.jenkinsProject, w.jenkinsBuild, w.filterFunc)
	if err != nil {
		return false, fmt.Errorf("failed to check for newer builds %w", err)
	}
	time.Sleep(20 * time.Second)
	if exists {
		return true, nil
	}
	return false, nil
}

// cleanupOldBuilds cleans up older jenkins builds by matching the ci message parameter. Returns error in case of fail
func (w *Worker) cleanupOldBuilds() error {
	klog.V(2).Infof("cleaning up old jenkins builds with matching ci message")
	err := jenkins.CleanupOldBuilds(w.jenkinsURL, w.jenkinsUser, w.jenkinsPassword, w.jenkinsProject, w.jenkinsBuild, w.filterFunc)
	if err != nil {
		return fmt.Errorf("failed to cleanup old builds %w", err)
	}
	return nil
}

//initQueues initializes rabbitmq queues used by worker. Returns error in case of fail
func (w *Worker) initQueues() error {
	if w.receiveQueue != nil {
		klog.V(2).Infof("initialising queues for worker")
		err := w.receiveQueue.Init()
		if err != nil {
			return fmt.Errorf("failed to initialize rcv queue %w", err)
		}
	}
	return nil
}

//run calls test by iterating over ssh nodes or calls test without a node if no ssh nodes
//returns success and error
func (w *Worker) run() (bool, error) {
	status := true
	var err error
	if w.sshNodes != nil {
		klog.V(2).Infof("found ssh nodes, iterating over the list")
		klog.V(4).Infof("ssh nodes looks like %#v", w.sshNodes)
		for _, nd := range w.sshNodes.Nodes {
			success, err := w.test(&nd)
			if err != nil {
				return false, err
			}
			if status {
				status = success
			}
		}
	} else {
		klog.V(2).Infof("running test locally on slave")
		status, err = w.test(nil)
		if err != nil {
			return false, err
		}
	}
	return status, nil
}

//Run runs the worker and returns error if any.
func (w *Worker) Run() (bool, error) {
	var success bool
	w.initFilterFunc()
	exists, err := w.checkForNewerBuilds()
	if err != nil {
		return false, err
	}
	if exists {
		return false, fmt.Errorf("newer build found, not moving forward")
	}
	go func() {
		for {
			exists1, _ := w.checkForNewerBuilds()
			if exists1 {
				log.Fatalf("found newer build, cancelling current build")
			}
			time.Sleep(2 * time.Minute)
		}
	}()
	if err = w.cleanupOldBuilds(); err != nil {
		return false, err
	}
	w.printBuildInfo()
	if err := w.initQueues(); err != nil {
		return false, err
	}
	err = w.sendCancelMessage()
	if err != nil {
		return false, fmt.Errorf("failed to send cancel message %w", err)
	}
	if err := w.sendBuildInfo(); err != nil {
		return false, fmt.Errorf("failed to send build info %w", err)
	}
	success, err = w.run()
	if err != nil {
		return false, fmt.Errorf("failed to run tests %w", err)
	}
	fmt.Printf("Success : %t\n", success)
	if err := w.sendStatusMessage(success); err != nil {
		return false, fmt.Errorf("failed to send status message %w", err)
	}
	if err := w.sendFinalizeMessage(); err != nil {
		return false, fmt.Errorf("failed to send finalize message %w", err)
	}
	err = w.psb.FlushToQueue()
	if err != nil {
		return false, err
	}
	return success, nil
}

//Shutdown shuts down the worker and returns error if any
func (w *Worker) Shutdown() error {
	if w.receiveQueue != nil {
		return w.receiveQueue.Shutdown(false)
	}
	return nil
}
