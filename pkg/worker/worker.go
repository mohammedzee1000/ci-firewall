package worker

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/mohammedzee1000/ci-firewall/pkg/executor"
	"github.com/mohammedzee1000/ci-firewall/pkg/jenkins"
	"github.com/mohammedzee1000/ci-firewall/pkg/messages"
	"github.com/mohammedzee1000/ci-firewall/pkg/node"
	"github.com/mohammedzee1000/ci-firewall/pkg/printstreambuffer"
	"github.com/mohammedzee1000/ci-firewall/pkg/queue"
	"github.com/mohammedzee1000/ci-firewall/pkg/util"
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
		w.receiveQueue = queue.NewAMQPQueue(wo.AMQPURI, wo.CIMessage.RcvIdent)
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
				if jcim.Kind == w.ciMessage.Kind && jcim.RcvIdent == w.ciMessage.RcvIdent && jcim.RepoURL == w.ciMessage.RepoURL && jcim.Target == w.ciMessage.Target && jcim.RunScript == w.ciMessage.RunScript && jcim.SetupScript == w.ciMessage.SetupScript && jcim.RunScriptURL == w.ciMessage.RunScriptURL && jcim.MainBranch == w.ciMessage.MainBranch && jcim.JenkinsProject == w.ciMessage.JenkinsProject {
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

func (w *Worker) sendCancelMessage() error {
	if w.receiveQueue != nil {
		klog.V(2).Infof("sending cancel message in order to ensure requester stops following any older builds")
		return w.receiveQueue.Publish(false, messages.NewCancelMessage(w.jenkinsBuild, w.jenkinsProject))
	}
	return nil
}

//sendBuildInfo sends information about the build. Returns error in case of fail
func (w *Worker) sendBuildInfo() error {
	if w.receiveQueue != nil {
		klog.V(2).Infof("publishing build information on rcv queue")
		return w.receiveQueue.Publish(false, messages.NewBuildMessage(w.jenkinsBuild, w.jenkinsProject))
	}
	return nil
}

//printAndStreamLog prints a the logs to the PrintStreamBuffer. Returns error in case of fail
func (w *Worker) printAndStreamLog(tags []string, msg string) error {
	err := w.psb.Print(fmt.Sprintf("%v %s", tags, msg), false, w.stripANSIColor)
	if err != nil {
		return fmt.Errorf("failed to stream log message %w", err)
	}
	return nil
}

func (w *Worker) handleCommandError(tags []string, err error) error {
	err1 := w.psb.Print(fmt.Sprintf("%v Command Error %s, failing gracefully", tags, err), true, w.stripANSIColor)
	if err1 != nil {
		return fmt.Errorf("failed to stream command error message %w", err)
	}
	return nil
}

//printAndStreamInfo prints and streams an info msg
func (w *Worker) printAndStreamInfo(tags []string, info string) error {
	toprint := fmt.Sprintf("%v !!!%s!!!\n", tags, info)
	return w.psb.Print(toprint, true, w.stripANSIColor)
}

func (w *Worker) printAndStreamErrors(tags []string, errlist []error) error {
	errMsg := "List of errors below:\n\n"
	for _, e := range errlist {
		errMsg = fmt.Sprintf(" - %s\n", e.Error())
	}
	return w.psb.Print(errMsg, true, w.stripANSIColor)
}

//printAndStreamCommand print and streams a command. Returns error in case of fail
func (w *Worker) printAndStreamCommand(tags []string, cmdArgs []string) error {
	return w.psb.Print(fmt.Sprintf("%v Executing command %v\n", tags, cmdArgs), true, w.stripANSIColor)
}

//runCommand runs cmd on ex the Executor in the workDir and returns success and error
func (w *Worker) runCommand(oldsuccess bool, ex executor.Executor, workDir string, cmd []string) (bool, error) {
	ctags := ex.GetTags()
	var errList []error
	success := true
	w.printAndStreamCommand(ctags, cmd)
	if oldsuccess {
		klog.V(4).Infof("injected env vars look like %#v", w.envVars)
		retryBackOff := w.retryLoopBackOff
		//keep retrying for ever incrementing retry (internal break conditions present)
		for retry := 1; ; retry++ {
			// if retry > 1 then we probably failed the last attempt
			if retry > 1 {
				success = false
				w.printAndStreamInfo(ctags, "attempt failed due to executor error")
				// we want to do retry loop backoff for all attempts greater than one until the last attempt
				if retry <= w.retryLoopCount {
					retryBackOff = retryBackOff + w.retryLoopBackOff
					w.printAndStreamInfo(ctags, fmt.Sprintf("backing of for %s before retrying", retryBackOff))
					time.Sleep(retryBackOff)
				}
			}
			// if the last attempt was done and was not successful, then we have failed
			// Note: this handles case where retry loop count was given as 1 as well as 1 >= 1 but success is true (see initialization above)
			if retry > w.retryLoopCount && !success {
				w.printAndStreamErrors(ctags, errList)
				return false, fmt.Errorf("failed due to errors, aborting %v", errList)
			}
			w.printAndStreamInfo(ctags, fmt.Sprintf("Attempt %d", retry))
			rdr, err := ex.InitCommand(workDir, cmd, util.EnvMapCopy(w.envVars), w.tags)
			if err != nil {
				errList = append(errList, fmt.Errorf("failed to initialize executor %w", err))
				continue
			}
			defer ex.Close()
			done := make(chan error)
			go func(done chan error) {
				for {
					data, err := rdr.ReadString('\n')
					if err != nil {
						if err != io.EOF {
							done <- fmt.Errorf("error while reading from buffer %w", err)
						}
						if len(data) > 0 {
							w.printAndStreamLog(ctags, data)
						}
						break
					}
					w.printAndStreamLog(ctags, data)
					time.Sleep(25 * time.Millisecond)
				}
				done <- nil
			}(done)
			// if start or wait error out, then we record the error and move on to next attempt
			err = ex.Start()
			if err != nil {
				errList = append(errList, fmt.Errorf("failed to start executing command %w", err))
				continue
			}
			err = <-done
			if err != nil {
				errList = append(errList, err)
				continue
			}
			success, err = ex.Wait()
			if err != nil {
				errList = append(errList, fmt.Errorf("failed to wait for command completion %w", err))
				continue
			}
			err = w.psb.FlushToQueue()
			if err != nil {
				return false, fmt.Errorf("failed to flush %w", err)
			}
			// if we have reached this point, then we do not have any executor errors, so return success status
			return success, nil
		}
	}
	w.printAndStreamInfo(ex.GetTags(), "previous command failed or skipped, skipping")
	return false, nil
}

func (w *Worker) setupGit(oldstatus bool, ex executor.Executor, repoDir string) (bool, error) {
	if oldstatus {
		var status bool
		var err error
		if w.gitUser != "" && w.gitEmail != "" {
			klog.V(2).Infof("configuring git user and git email")
			klog.V(3).Infof("user %s with email %s", w.gitUser, w.gitEmail)
			status, err = w.runCommand(true, ex, repoDir, []string{"git", "config", "user.name", fmt.Sprintf("\"%s\"", w.gitUser)})
			if err != nil {
				return false, fmt.Errorf("failed to set git user %w", err)
			}
			status, err = w.runCommand(status, ex, repoDir, []string{"git", "config", "user.email", fmt.Sprintf("\"%s\"", w.gitEmail)})
		}
	} else {
		return false, nil
	}
	return true, nil
}

//setupTests sets up testing using Executor ex, in workDir the workdirectory and repoDir the repo clone location. Returns success and error.
func (w *Worker) setupTests(ex executor.Executor, workDir, repoDir string) (bool, error) {
	var err error
	var chkout string
	klog.V(2).Infof("setting up tests")
	//Remove any existing workdir of same name, ussually due to termination of jobs
	status, err := w.runCommand(true, ex, "", []string{"rm", "-rf", workDir})
	if err != nil {
		w.handleCommandError(ex.GetTags(), err)
	}
	//create new workdir and repodir
	status, err = w.runCommand(status, ex, "", []string{"mkdir", "-p", repoDir})
	if err != nil {
		return false, fmt.Errorf("failed to create workdir %w", err)
	}
	klog.V(2).Infof("cloning repo and checking out the target")
	status, err = w.runCommand(status, ex, "", []string{"git", "clone", w.ciMessage.RepoURL, repoDir})
	if err != nil {
		return false, fmt.Errorf("git clone failed %w", err)
	}
	status, err = w.setupGit(status, ex, repoDir)
	if err != nil {
		return false, fmt.Errorf("failed to setup git %w", err)
	}
	if w.ciMessage.Kind == messages.RequestTypePR {
		klog.V(2).Infof("checking out PR and merging it with the main branch")
		klog.V(3).Infof("PR %s and main branch %s", w.ciMessage.Target, w.ciMessage.MainBranch)
		chkout = fmt.Sprintf("pr%s", w.ciMessage.Target)
		pulltgt := fmt.Sprintf("pull/%s/head:%s", w.ciMessage.Target, chkout)
		status1, err := w.runCommand(status, ex, repoDir, []string{"git", "fetch", "-v", "origin", pulltgt})
		if err != nil {
			return false, fmt.Errorf("failed to fetch pr no %s, are you sure it exists in repo %s %w", w.ciMessage.Target, w.ciMessage.RepoURL, err)
		}
		if !status1 {
			fmt.Printf("couldn't find remote ref for pr no %s, running tests on main branch", w.ciMessage.Target)
		}
		status, err = w.runCommand(true, ex, repoDir, []string{"git", "checkout", w.ciMessage.MainBranch})
		if err != nil {
			return false, fmt.Errorf("failed to switch to main branch %w", err)
		}
		if status1 {
			status, err = w.runCommand(status, ex, repoDir, []string{"git", "merge", chkout, "--no-edit"})
			if err != nil {
				return false, fmt.Errorf("failed to fast forward merge %w", err)
			}
		}
	} else if w.ciMessage.Kind == messages.RequestTypeBranch {
		klog.V(2).Infof("checkout out branch")
		chkout = w.ciMessage.Target
		//4 checkout
		status, err = w.runCommand(status, ex, repoDir, []string{"git", "checkout", chkout})
		if err != nil {
			return false, fmt.Errorf("failed to checkout %w", err)
		}
	} else if w.ciMessage.Kind == messages.RequestTypeTag {
		klog.V(2).Infof("checking out git tag")
		chkout = fmt.Sprintf("tags/%s", w.ciMessage.Target)
		//4 checkout
		status, err = w.runCommand(status, ex, repoDir, []string{"git", "checkout", chkout})
		if err != nil {
			return false, fmt.Errorf("failed to checkout %w", err)
		}
	} else {
		return false, fmt.Errorf("invalid kind parameter %s. Must be one of %s, %s or %s", w.ciMessage.Kind, messages.RequestTypePR, messages.RequestTypeBranch, messages.RequestTypeTag)
	}
	return status, nil
}

//runTests runs tests using executor ex and repoDir the repo clone location. If oldstatus is false, it is skipped
func (w *Worker) runTests(oldstatus bool, ex executor.Executor, repoDir string) (bool, error) {
	var err error
	if oldstatus {
		status := true
		klog.V(2).Infof("setting up test command")
		//1 Setup the runCmd based on if setup script and run script
		var runCmd string
		if w.ciMessage.SetupScript != "" {
			klog.Infof("setup script detected, adding to command")
			runCmd = fmt.Sprint(". ", w.ciMessage.SetupScript, " && ")
		}
		klog.V(2).Infof("adding run script to command")
		runCmd = fmt.Sprint(runCmd, ". ", w.ciMessage.RunScript)
		runCmd = fmt.Sprintf("\"%s\"", runCmd)
		//2 Download runscript, if provided
		if w.ciMessage.RunScriptURL != "" {
			klog.V(2).Infof("downloading run script for a url")
			status, err = w.runCommand(status, ex, repoDir, []string{"curl", "-kLo", w.ciMessage.RunScript, w.ciMessage.RunScriptURL})
			if err != nil {
				return false, fmt.Errorf("failed to download run script")
			}
		}

		//3 run run cmd
		status, err = w.runCommand(status, ex, repoDir, []string{"sh", "-c", runCmd})
		if err != nil {
			return false, fmt.Errorf("failed to run run script")
		}
		return status, nil
	}
	w.printAndStreamInfo(ex.GetTags(), "setup failed, skipping")
	return false, nil
}

//tearDownTests cleanups up using Executor ex in workDir the workDirectory and returns success and error
//if oldsuccess is false, then this is skipped
func (w *Worker) tearDownTests(oldsuccess bool, ex executor.Executor, workDir string) (bool, error) {
	klog.V(2).Infof("tearing down test env")
	if oldsuccess {
		status, err := w.runCommand(oldsuccess, ex, "", []string{"rm", "-rf", workDir})
		if err != nil {
			return false, fmt.Errorf("failed to remove workdir %w", err)
		}
		return status, nil
	}
	w.printAndStreamInfo(ex.GetTags(), "run failed, skipping")
	return false, nil
}

//test runs the tests on a node. If node is nill LocalExecutor is used, otherwise SSHExecutor is used.
//returns success and error
func (w *Worker) test(nd *node.Node) (bool, error) {
	var err error
	var ex executor.Executor
	baseWorkDir := util.GetBaseWorkDir(w.ciMessage.RcvIdent)
	instanceWorkDir := filepath.Join(baseWorkDir, util.GetInstanceWorkdirName())
	repoDir := filepath.Join(instanceWorkDir, w.repoDir)
	if nd != nil {
		klog.V(2).Infof("no node specified, creating LocalExecutor")
		ex, err = executor.NewNodeSSHExecutor(nd)
		if err != nil {
			return false, fmt.Errorf("failed to setup ssh executor %w", err)
		}
		w.printAndStreamInfo(ex.GetTags(), fmt.Sprintf("running tests on node %s via ssh", nd.Name))
	} else {
		klog.V(2).Infof("node specified, creating node executor")
		klog.V(4).Infof("node information looks like %#v", nd)
		ex = executor.NewLocalExecutor()
		w.printAndStreamInfo(ex.GetTags(), "running tests locally")
	}
	status, err := w.setupTests(ex, baseWorkDir, repoDir)
	if err != nil {
		return false, fmt.Errorf("setup failed %w", err)
	}
	status, err = w.runTests(status, ex, repoDir)
	if err != nil {
		return false, fmt.Errorf("failed to run the tests %w", err)
	}
	status, err = w.tearDownTests(status, ex, baseWorkDir)
	if err != nil {
		return false, fmt.Errorf("failed cleanup %w", err)
	}
	return status, nil
}

//run calls test by iterating over sshnodes or calls test without a node if no sshnodes
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

//sendStatusMessage sends the status message over queue, based on success value
func (w *Worker) sendStatusMessage(success bool) error {
	sm := messages.NewStatusMessage(w.jenkinsBuild, success, w.jenkinsProject)
	klog.V(2).Infof("sending status message")
	if w.receiveQueue != nil {
		return w.receiveQueue.Publish(false, sm)
	}
	return nil
}

func (w *Worker) sendFinalizeMessage() error {
	klog.V(2).Infof("sending final message")
	if w.receiveQueue != nil && w.final {
		return w.receiveQueue.Publish(false, messages.NewFinalMessage(w.jenkinsBuild, w.jenkinsProject))
	}
	return nil
}

func (w *Worker) printBuildInfo() {
	fmt.Printf("!!!Build for Kind: %s Target: %s!!!\n", w.ciMessage.Kind, w.ciMessage.Target)
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
