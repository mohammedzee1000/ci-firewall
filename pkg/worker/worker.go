package worker

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

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
	rcvq            *queue.AMQPQueue
	jenkinsProject  string
	jenkinsBuild    int
	jenkinsURL      string
	jenkinsUser     string
	jenkinsPassword string
	cimsgenv        string
	cimsg           *messages.RemoteBuildRequestMessage
	envVars         map[string]string
	envFile         string
	repoDir         string
	sshNodes        *node.NodeList
	psb             *printstreambuffer.PrintStreamBuffer
}

//NewWorker creates a new worker struct. if standalone is true, then rabbitmq is not used for communication with requestor.
//Instead cimsg must be provided manually(see readme). amqpURI is full uri (including username and password) of rabbitmq server.
//jenkinsURL, jenkinsUser, jenkinsPassword, jenkinsProject are info related to jenkins (robot account is used for cancelling
//older builds by matching job parameter cienvmsg). cimsg is parsed CI message. to provide nessasary info to worker and also match
//and cleanup older jenkins jobs. envVars are envs to be exposed to the setup and run scripts . jenkinsBuild is current jenkins build
//number. psbSize is max buffer size for PrintStreamBuffer and sshNode is a parsed sshnodefile (see readme)
func NewWorker(standalone bool, amqpURI, jenkinsURL, jenkinsUser, jenkinsPassword, jenkinsProject string, cimsgenv string, cimsg *messages.RemoteBuildRequestMessage, envVars map[string]string, jenkinsBuild int, psbsize int, sshNodes *node.NodeList) *Worker {
	w := &Worker{
		rcvq:            nil,
		cimsg:           cimsg,
		jenkinsProject:  jenkinsProject,
		jenkinsBuild:    jenkinsBuild,
		jenkinsURL:      jenkinsURL,
		jenkinsUser:     jenkinsUser,
		jenkinsPassword: jenkinsPassword,
		envVars:         envVars,
		envFile:         "env.sh",
		repoDir:         "repo",
		sshNodes:        sshNodes,
	}
	if !standalone {
		w.rcvq = queue.NewAMQPQueue(amqpURI, cimsg.RcvIdent)
	}
	w.envVars[scriptIdentity] = strings.ToLower(fmt.Sprintf("%s%s%s", jenkinsProject, cimsg.Kind, cimsg.Target))
	w.psb = printstreambuffer.NewPrintStreamBuffer(w.rcvq, psbsize, w.jenkinsBuild)
	return w
}

// cleanupOldBuilds cleans up older jenkins builds by matching the ci message parameter. Returns error in case of fail
func (w *Worker) cleanupOldBuilds() error {
	err := jenkins.CleanupOldBuilds(w.jenkinsURL, w.jenkinsUser, w.jenkinsPassword, w.jenkinsProject, w.jenkinsBuild, func(params map[string]string) bool {
		for k, v := range params {
			if k == w.cimsgenv {
				//v is the cimsg of this job
				jcim := messages.NewRemoteBuildRequestMessage("", "", "", "", "", "", "", "")
				json.Unmarshal([]byte(v), jcim)
				if jcim.Kind == w.cimsg.Kind && jcim.RcvIdent == w.cimsg.RcvIdent && jcim.RepoURL == w.cimsg.RepoURL && jcim.Target == w.cimsg.Target && jcim.RunScript == w.cimsg.RunScript && jcim.SetupScript == w.cimsg.SetupScript && jcim.RunScriptURL == w.cimsg.RunScriptURL && jcim.MainBranch == w.cimsg.MainBranch {
					return true
				}
			}
		}
		return false
	})
	if err != nil {
		return fmt.Errorf("failed to cleanup old builds %w", err)
	}
	return nil
}

//initQueues initializes rabbitmq queues used by worker. Returns error in case of fail
func (w *Worker) initQueues() error {
	if w.rcvq != nil {
		err := w.rcvq.Init()
		if err != nil {
			return fmt.Errorf("failed to initialize rcv queue %w", err)
		}
	}
	return nil
}

//sendBuildInfo sends information about the build. Returns error in case of fail
func (w *Worker) sendBuildInfo() error {
	if w.rcvq != nil {
		return w.rcvq.Publish(false, messages.NewBuildMessage(w.jenkinsBuild))
	}
	return nil
}

//printAndStreamLog prints a the logs to the PrintStreamBuffer. Returns error in case of fail
func (w *Worker) printAndStreamLog(msg string) error {
	err := w.psb.Print(msg)
	if err != nil {
		return fmt.Errorf("failed to stream log message %w", err)
	}
	return nil
}

//printAndStreamInfo prints and streams an info msg
func (w *Worker) printAndStreamInfo(info string) error {
	return w.psb.Println(info)
}

//printAndStreamCommand print and streams a command. Returns error in case of fail
func (w *Worker) printAndStreamCommand(cmdArgs []string) error {
	return w.printAndStreamInfo(fmt.Sprintf("Executing command %v", cmdArgs))
}

//runCommand runs cmd on ex the Executor in the workDir and returns success and error
func (w *Worker) runCommand(oldsuccess bool, ex executor.Executor, workDir string, cmd []string) (bool, error) {
	w.printAndStreamCommand(cmd)
	if oldsuccess {
		rdr, err := ex.InitCommand(workDir, cmd, util.EnvMapCopy(w.envVars))
		if err != nil {
			return false, fmt.Errorf("failed to initialize executor %w", err)
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
					break
				}
				w.printAndStreamLog(data)
			}
			done <- nil
		}(done)
		err = ex.Start()
		if err != nil {
			return false, fmt.Errorf("failed to run command %w", err)
		}
		err = <-done
		if err != nil {
			return false, err
		}
		ex.Wait()
		err = w.psb.Flush()
		if err != nil {
			return false, err
		}
		if ex.ExitCode() != 0 {
			return false, nil
		}
		return true, nil
	}
	w.printAndStreamInfo("previous command failed or skipped, skipping")
	return false, nil
}

//setupTests sets up testing using Executor ex, in workDir the workdirectory and repoDir the repo clone location. Returns success and error.
func (w *Worker) setupTests(ex executor.Executor, workDir, repoDir string) (bool, error) {
	var status bool
	var err error
	var chkout string
	//Remove any existing workdir of same name, ussually due to termination of jobs
	status, err = w.runCommand(true, ex, "", []string{"rm", "-rf", workDir})
	if err != nil {
		return false, fmt.Errorf("failed to delete workdir %w", err)
	}
	//create new workdir and repodir
	status, err = w.runCommand(status, ex, "", []string{"mkdir", "-p", repoDir})
	if err != nil {
		return false, fmt.Errorf("failed to create workdir %w", err)
	}
	status, err = w.runCommand(status, ex, "", []string{"git", "clone", w.cimsg.RepoURL, repoDir})
	if err != nil {
		return false, fmt.Errorf("failed to clone repo %w", err)
	}
	if w.cimsg.Kind == messages.RequestTypePR {
		chkout = fmt.Sprintf("pr%s", w.cimsg.Target)
		status, err = w.runCommand(status, ex, repoDir, []string{"git", "fetch", "origin", fmt.Sprintf("pull/%s/head:%s", w.cimsg.Target, chkout)})
		if err != nil {
			return false, fmt.Errorf("failed to fetch pr %w", err)
		}
		status, err = w.runCommand(status, ex, repoDir, []string{"git", "checkout", w.cimsg.MainBranch})
		if err != nil {
			return false, fmt.Errorf("failed to switch to main branch %w", err)
		}
		status, err = w.runCommand(status, ex, repoDir, []string{"git", "merge", chkout, "--no-edit"})
		if err != nil {
			return false, fmt.Errorf("failed to fast forward merge %w", err)
		}
	} else {
		if w.cimsg.Target == messages.RequestTypeBranch {
			chkout = w.cimsg.Target
		} else if w.cimsg.Kind == messages.RequestTypeTag {
			chkout = fmt.Sprintf("tags/%s", w.cimsg.Target)
		}
		//4 checkout
		status, err = w.runCommand(status, ex, repoDir, []string{"git", "checkout", chkout})
		if err != nil {
			return false, fmt.Errorf("failed to checkout %w", err)
		}
	}
	return status, nil
}

//runTests runs tests using executor ex and repoDir the repo clone location. If oldstatus is false, it is skipped
func (w *Worker) runTests(oldstatus bool, ex executor.Executor, repoDir string) (bool, error) {
	var status bool
	var err error
	if oldstatus {
		//1 run the setup script, if it is provided
		if w.cimsg.SetupScript != "" {
			status, err = w.runCommand(status, ex, repoDir, []string{"sh", w.cimsg.SetupScript})
			if err != nil {
				return false, fmt.Errorf("failed to run setup script")
			}
		}
		//2 Download runscript, if provided
		if w.cimsg.RunScriptURL != "" {
			status, err = w.runCommand(status, ex, repoDir, []string{"curl", "-kLo", w.cimsg.RunScript, w.cimsg.RunScriptURL})
			if err != nil {
				return false, fmt.Errorf("failed to download run script")
			}
		}

		//3 run run script
		status, err = w.runCommand(status, ex, repoDir, []string{"sh", w.cimsg.RunScript})
		if err != nil {
			return false, fmt.Errorf("failed to run run script")
		}
	} else {
		w.printAndStreamLog("setup failed, skipping")
	}
	return status, nil
}

//tearDownTests cleanups up using Executor ex in workDir the workDirectory and returns success and error
//if oldsuccess is false, then this is skipped
func (w *Worker) tearDownTests(oldsuccess bool, ex executor.Executor, workDir string) (bool, error) {
	var status bool
	var err error
	if oldsuccess {
		status, err = w.runCommand(oldsuccess, ex, "", []string{"rm", "-rf", workDir})
		if err != nil {
			return false, fmt.Errorf("failed to remove workdir %w", err)
		}
	} else {
		w.printAndStreamLog("run failed, skipping")
	}
	return status, nil
}

//test runs the tests on a node. If node is nill LocalExecutor is used, otherwise SSHExecutor is used.
//returns success and error
func (w *Worker) test(nd *node.Node) (bool, error) {
	var status bool
	var err error
	var ex executor.Executor
	workDir := strings.ReplaceAll(w.cimsg.RcvIdent, ".", "_")
	repoDir := filepath.Join(workDir, w.repoDir)
	if nd != nil {
		ex, err = executor.NewNodeSSHExecutor(nd)
		if err != nil {
			return false, fmt.Errorf("failed to setup ssh executor %w", err)
		}
		w.printAndStreamInfo(fmt.Sprintf("!!!running tests on node %s via ssh!!!", nd.Name))
	} else {
		ex = executor.NewLocalExecutor()
		w.printAndStreamInfo("running tests locally")
	}
	status, err = w.setupTests(ex, workDir, repoDir)
	if err != nil {
		return false, fmt.Errorf("setup failed %w", err)
	}
	status, err = w.runTests(status, ex, repoDir)
	if err != nil {
		return false, fmt.Errorf("failed to run the tests %w", err)
	}
	status, err = w.tearDownTests(status, ex, workDir)
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
		status, err = w.test(nil)
		if err != nil {
			return false, err
		}
	}
	return status, nil
}

//sendStatusMessage sends the status message over queue, based on success value
func (w *Worker) sendStatusMessage(success bool) error {
	if w.rcvq != nil {
		return w.rcvq.Publish(false, messages.NewStatusMessage(w.jenkinsBuild, success))
	}
	return nil
}

//Run runs the worker and returns error if any.
func (w *Worker) Run() error {
	var success bool
	if err := w.cleanupOldBuilds(); err != nil {
		return err
	}
	if err := w.initQueues(); err != nil {
		return err
	}
	if err := w.sendBuildInfo(); err != nil {
		return fmt.Errorf("failed to send build info %w", err)
	}
	success, err := w.run()
	if err != nil {
		return fmt.Errorf("failed to run tests %w", err)
	}
	fmt.Printf("Success : %t\n", success)
	if err := w.sendStatusMessage(success); err != nil {
		return fmt.Errorf("failed to send status message %w", err)
	}
	err = w.psb.Flush()
	if err != nil {
		return err
	}
	return nil
}

//Shutdown shuts down the worker and returns error if any
func (w *Worker) Shutdown() error {
	if w.rcvq != nil {
		return w.rcvq.Shutdown()
	}
	return nil
}
