package worker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
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

func NewWorker(amqpURI, jenkinsURL, jenkinsUser, jenkinsPassword, jenkinsProject string, cimsgenv string, cimsg *messages.RemoteBuildRequestMessage, envVars map[string]string, jenkinsBuild int, sshNodes *node.NodeList) *Worker {
	w := &Worker{
		rcvq:            queue.NewAMQPQueue(amqpURI, cimsg.RcvIdent),
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
	w.psb = printstreambuffer.NewPrintStreamBuffer(w.rcvq, 10, w.jenkinsBuild)
	return w
}

// 1
func (w *Worker) cleanupOldBuilds() error {
	err := jenkins.CleanupOldBuilds(w.jenkinsURL, w.jenkinsUser, w.jenkinsPassword, w.jenkinsProject, w.jenkinsBuild, func(params map[string]string) bool {
		for k, v := range params {
			if k == w.cimsgenv {
				//v is the cimsg of this job
				jcim := messages.NewRemoteBuildRequestMessage("", "", "", "", "", "")
				json.Unmarshal([]byte(v), jcim)
				if jcim.Kind == w.cimsg.Kind && jcim.RcvIdent == w.cimsg.RcvIdent && jcim.RepoURL == w.cimsg.RepoURL && jcim.Target == w.cimsg.Target && jcim.RunScript == w.cimsg.RunScript && jcim.SetupScript == w.cimsg.SetupScript {
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

// 2
func (w *Worker) initQueues() error {
	err := w.rcvq.Init()
	if err != nil {
		return fmt.Errorf("failed to initialize rcv queue %w", err)
	}
	return nil
}

// 3
func (w *Worker) sendBuildInfo() error {
	return w.rcvq.Publish(false, messages.NewBuildMessage(w.jenkinsBuild))
}

func (w *Worker) printAndStream(msg string) error {
	err := w.psb.Println(msg, false)
	if err != nil {
		return fmt.Errorf("failed to stream log message %w", err)
	}
	return nil
}

func (w *Worker) printAndStreamCommand(cmdArgs []string) error {
	return w.psb.Println(fmt.Sprintf("Executing command %v", cmdArgs), true)
}

func (w *Worker) printAndStreamCommandString(cmdArgs string) error {
	return w.psb.Println(fmt.Sprintf("Executing command [%s]", cmdArgs), true)
}

func (w *Worker) runCommand(oldsuccess bool, ex executor.Executor) (bool, error) {
	var err error
	defer ex.Close()
	if oldsuccess {
		ex.SetEnvs(util.EnvMapCopy(w.envVars))
		done := make(chan error)
		ex.ShortStderrToStdOut()
		r, _ := ex.StdoutPipe()
		scanner := bufio.NewScanner(r)
		go func(done chan error) {
			for scanner.Scan() {
				line := scanner.Text()
				err1 := w.printAndStream(line)
				if err1 != nil {
					done <- err1
				}
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
	w.printAndStream("previous command failed or skipped, skipping")
	return false, nil
}

func (w *Worker) runTestsLocally() (bool, error) {
	var success bool
	var status bool
	var err error
	var chkout string

	//local executor
	w.printAndStream("running tests locally")
	//1. clone the repo
	cmd1 := []string{"git", "clone", w.cimsg.RepoURL, w.repoDir}
	w.printAndStreamCommand(cmd1)
	ex1 := executor.NewLocalExecutor(cmd1)
	status, err = w.runCommand(true, ex1)
	if err != nil {
		return false, fmt.Errorf("failed to clone repo %w", err)
	}
	//2. change to repodir
	os.Chdir(w.repoDir)
	//3. Fetch if needed
	if w.cimsg.Kind == messages.RequestTypePR {
		chkout = fmt.Sprintf("pr%s", w.cimsg.Target)
		cmd3 := []string{"git", "fetch", "origin", fmt.Sprintf("pull/%s/head:%s", w.cimsg.Target, chkout)}
		w.printAndStreamCommand(cmd3)
		ex3 := executor.NewLocalExecutor(cmd3)
		status, err = w.runCommand(status, ex3)
		if err != nil {
			return false, fmt.Errorf("failed to fetch pr %w", err)
		}
	} else if w.cimsg.Target == messages.RequestTypeBranch {
		chkout = w.cimsg.Target
	} else if w.cimsg.Kind == messages.RequestTypeTag {
		chkout = fmt.Sprintf("tags/%s", w.cimsg.Target)
	}
	//4 checkout
	cmd4 := []string{"git", "checkout", chkout}
	w.printAndStreamCommand(cmd4)
	ex4 := executor.NewLocalExecutor(cmd4)
	status, err = w.runCommand(status, ex4)
	if err != nil {
		return false, fmt.Errorf("failed to checkout %w", err)
	}
	//5 run the setup script, if it is provided
	if w.cimsg.SetupScript != "" {
		cmd5 := []string{"sh", w.cimsg.SetupScript}
		w.printAndStreamCommand(cmd5)
		ex5 := executor.NewLocalExecutor(cmd5)
		status, err = w.runCommand(status, ex5)
		if err != nil {
			return false, fmt.Errorf("failed to run setup script")
		}
	}
	//6 run run script
	cmd6 := []string{"sh", w.cimsg.RunScript}
	w.printAndStreamCommand(cmd6)
	ex6 := executor.NewLocalExecutor(cmd6)
	status, err = w.runCommand(status, ex6)
	if err != nil {
		return false, fmt.Errorf("failed to run run script")
	}
	//2C. getout of repodir
	os.Chdir("..")
	success = status

	return success, nil
}

func (w *Worker) runTestsOnNode(nd *node.Node) (bool, error) {
	var success bool
	var status bool
	var chkout string

	// If we have node, then we need to use node executor
	if nd != nil {
		//node executor
		workDir := strings.ReplaceAll(w.cimsg.RcvIdent, ".", "_")
		w.printAndStream(fmt.Sprintf("running tests on node %s via ssh", nd.Name))

		repoDir := filepath.Join(workDir, w.repoDir)
		//Remove any existing workdir of same name, ussually due to termination of jobs
		cmdd1 := []string{"rm", "-rf", workDir}
		w.printAndStreamCommand(cmdd1)
		exd1, err := executor.NewNodeSSHExecutor(nd, "", cmdd1)
		if err != nil {
			return false, fmt.Errorf("unable to create ssh executor %w", err)
		}
		status, err = w.runCommand(true, exd1)
		if err != nil {
			return false, fmt.Errorf("unable to cleanup workdir in ssh node %w", err)
		}
		//create new workdir and repodir
		cmdc1 := []string{"mkdir", "-p", repoDir}
		w.printAndStreamCommand(cmdc1)
		exc1, err := executor.NewNodeSSHExecutor(nd, "", cmdc1)
		if err != nil {
			return false, fmt.Errorf("unable to create ssh executor %w", err)
		}
		status, err = w.runCommand(true, exc1)
		if err != nil {
			return false, fmt.Errorf("unable to cleanup workdir in ssh node %w", err)
		}
		//run the tests
		//1. Clone the repo
		cmd1 := []string{"git", "clone", w.cimsg.RepoURL, repoDir}
		w.printAndStreamCommand(cmd1)
		ex1, err := executor.NewNodeSSHExecutor(nd, "", cmd1)
		if err != nil {
			return false, fmt.Errorf("unable to create ssh executor %w", err)
		}
		status, err = w.runCommand(status, ex1)
		if err != nil {
			return false, fmt.Errorf("unable to clone repo %w", err)
		}
		//2. fetch if needed
		if w.cimsg.Kind == messages.RequestTypePR {
			chkout = fmt.Sprintf("pr%s", w.cimsg.Target)
			cmd2 := []string{"git", "fetch", "origin", fmt.Sprintf("pull/%s/head:%s", w.cimsg.Target, chkout)}
			w.printAndStreamCommand(cmd2)
			ex3, err := executor.NewNodeSSHExecutor(nd, repoDir, cmd2)
			if err != nil {
				return false, fmt.Errorf("unable to create ssh executor %w", err)
			}
			status, err = w.runCommand(status, ex3)
			if err != nil {
				return false, fmt.Errorf("failed to fetch pr %w", err)
			}
		} else if w.cimsg.Kind == messages.RequestTypeBranch {
			chkout = w.cimsg.Target
		} else if w.cimsg.Kind == messages.RequestTypeTag {
			chkout = fmt.Sprintf("tags/%s", w.cimsg.Target)
		}
		//3. Checkout target
		cmd4 := []string{"git", "checkout", chkout}
		w.printAndStreamCommand(cmd4)
		ex4, err := executor.NewNodeSSHExecutor(nd, repoDir, cmd4)
		if err != nil {
			return false, fmt.Errorf("unable to create ssh executor %w", err)
		}
		status, err = w.runCommand(status, ex4)
		if err != nil {
			return false, fmt.Errorf("failed to checkout %w", err)
		}
		//4. run the setup script, if it is provided
		if w.cimsg.SetupScript != "" {
			w.printAndStream("running setup script")
			cmd5 := []string{"sh", w.cimsg.SetupScript}
			w.printAndStreamCommand(cmd5)
			ex5, err := executor.NewNodeSSHExecutor(nd, repoDir, cmd5)
			if err != nil {
				return false, fmt.Errorf("unable to create ssh executor %w", err)
			}
			status, err = w.runCommand(status, ex5)
			if err != nil {
				return false, fmt.Errorf("failed to run setup script")
			}
		}
		//5. Run the run script
		cmd6 := []string{"sh", w.cimsg.RunScript}
		w.printAndStreamCommand(cmd6)
		ex6, err := executor.NewNodeSSHExecutor(nd, repoDir, cmd6)
		if err != nil {
			return false, fmt.Errorf("unable to create ssh executor %w", err)
		}
		status, err = w.runCommand(status, ex6)
		if err != nil {
			return false, fmt.Errorf("failed to run run script")
		}
		//remove workdir on success
		cmdd2 := []string{"rm", "-rf", workDir}
		w.printAndStreamCommand(cmdd2)
		exd2, err := executor.NewNodeSSHExecutor(nd, "", cmdd2)
		if err != nil {
			return false, fmt.Errorf("unable to create ssh executor %w", err)
		}
		status, err = w.runCommand(status, exd2)
		if err != nil {
			return false, fmt.Errorf("unable to cleanup workdir in ssh node %w", err)
		}
		success = status
	}
	return success, nil
}

// 4
func (w *Worker) test() (bool, error) {
	status := true
	var err error

	if w.sshNodes != nil {
		for _, nd := range w.sshNodes.Nodes {
			success, err := w.runTestsOnNode(&nd)
			if err != nil {
				return false, err
			}
			if status {
				status = success
			}
		}
	} else {
		status, err = w.runTestsLocally()
		if err != nil {
			return false, err
		}
	}
	// }
	return status, nil
}

// 5
func (w *Worker) sendStatusMessage(success bool) error {
	return w.rcvq.Publish(false, messages.NewStatusMessage(w.jenkinsBuild, success))
}

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
	success, err := w.test()
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

func (w *Worker) Shutdown() error {
	return w.rcvq.Shutdown()
}
