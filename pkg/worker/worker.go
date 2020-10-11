package worker

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mohammedzee1000/ci-firewall/pkg/executor"
	"github.com/mohammedzee1000/ci-firewall/pkg/jenkins"
	"github.com/mohammedzee1000/ci-firewall/pkg/messages"
	"github.com/mohammedzee1000/ci-firewall/pkg/node"
	"github.com/mohammedzee1000/ci-firewall/pkg/queue"
)

type Worker struct {
	rcvq            *queue.AMQPQueue
	jenkinsProject  string
	jenkinsBuild    int
	jenkinsURL      string
	jenkinsUser     string
	jenkinsPassword string
	repoURL         string
	kind            string
	target          string
	setupScript     string
	runScript       string
	envVars         map[string]string
	envFile         string
	repoDir         string
	sshNodes        *node.NodeList
}

func NewWorker(amqpURI, jenkinsURL, jenkinsUser, jenkinsPassword, jenkinsProject, kind, repoURL, target, setupScript, runScript, rcvQueueName string, envVars map[string]string, jenkinsBuild int, sshNodes *node.NodeList) *Worker {
	w := &Worker{
		rcvq:            queue.NewAMQPQueue(amqpURI, rcvQueueName),
		kind:            kind,
		repoURL:         repoURL,
		target:          target,
		runScript:       runScript,
		setupScript:     setupScript,
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
	return w
}

// 1
func (w *Worker) cleanupOldBuilds() error {
	err := jenkins.CleanupOldBuilds(w.jenkinsURL, w.jenkinsUser, w.jenkinsPassword, w.jenkinsProject, w.jenkinsBuild, func(params map[string]string) bool {
		counter := 0
		for k, v := range params {
			if k == messages.RequestParameterKind && v == w.kind {
				counter = counter + 1
			} else if k == messages.RequestParameterTarget && v == w.target {
				counter = counter + 1
			} else if k == messages.RequestParameterRunScript && v == w.runScript {
				counter = counter + 1
			}
		}
		if counter == 3 {
			return true
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
	fmt.Println(msg)
	lm := messages.NewLogsMessage(w.jenkinsBuild, msg)
	err := w.rcvq.Publish(
		false, lm,
	)
	if err != nil {
		return fmt.Errorf("failed to stream log message %w", err)
	}
	return nil
}

func (w *Worker) printAndStreamCommand(cmdArgs []string) error {
	return w.printAndStream(fmt.Sprintf("Executing command %v", cmdArgs))
}

func (w *Worker) printAndStreamCommandString(cmdArgs string) error {
	return w.printAndStream(fmt.Sprintf("Executing command [%s]", cmdArgs))
}

func (w *Worker) runCommand(oldsuccess bool, ex executor.Executor) (bool, error) {
	var err error
	defer ex.Close()
	if oldsuccess {
		//cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		ex.SetEnvs(w.envVars)
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
		if ex.ExitCode() != 0 {
			return false, nil
		}
		return true, nil
	}
	w.printAndStream("previous command failed or skipped, skipping")
	return false, nil
}

func (w *Worker) runTests(nd *node.Node) (bool, error) {
	var status bool
	var success bool
	var err error
	var chkout string

	// If we have node, then we need to use node executor
	if nd != nil {
		//node executor
		w.printAndStream(fmt.Sprintf("running tests on node %s via ssh", nd.Name))
		workDir := fmt.Sprintf("%s_%s", w.jenkinsProject, w.target)
		repoDir := filepath.Join(workDir, w.repoDir)
		//Remove any existing workdir of same name, ussually due to termination of jobs
		cmdd1 := []string{"rm", "-rf", workDir}
		w.printAndStreamCommand(cmdd1)
		exd1, err := executor.NewNodeSSHExecutor(nd, "", cmdd1)
		if err != nil {
			return false, fmt.Errorf("unable to create ssh executor %w", err)
		}
		success, err = w.runCommand(true, exd1)
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
		success, err = w.runCommand(true, exc1)
		if err != nil {
			return false, fmt.Errorf("unable to cleanup workdir in ssh node %w", err)
		}
		//run the tests
		//1. Clone the repo
		cmd1 := []string{"git", "clone", w.repoURL, w.repoDir}
		w.printAndStreamCommand(cmd1)
		ex1, err := executor.NewNodeSSHExecutor(nd, workDir, cmd1)
		if err != nil {
			return false, fmt.Errorf("unable to create ssh executor %w", err)
		}
		success, err = w.runCommand(success, ex1)
		if err != nil {
			return false, fmt.Errorf("unable to clone repo %w", err)
		}
		//2. fetch if needed
		if w.kind == messages.RequestTypePR {
			chkout = fmt.Sprintf("pr%s", w.target)
			cmd2 := []string{"git", "fetch", "origin", fmt.Sprintf("pull/%s/head:%s", w.target, chkout)}
			w.printAndStreamCommand(cmd2)
			ex3, err := executor.NewNodeSSHExecutor(nd, repoDir, cmd2)
			if err != nil {
				return false, fmt.Errorf("unable to create ssh executor %w", err)
			}
			success, err = w.runCommand(success, ex3)
			if err != nil {
				return false, fmt.Errorf("failed to fetch pr %w", err)
			}
		} else if w.kind == messages.RequestTypeBranch {
			chkout = w.target
		} else if w.kind == messages.RequestTypeTag {
			chkout = fmt.Sprintf("tags/%s", w.target)
		}
		//3. Checkout target
		cmd4 := []string{"git", "checkout", chkout}
		w.printAndStreamCommand(cmd4)
		ex4, err := executor.NewNodeSSHExecutor(nd, repoDir, cmd4)
		if err != nil {
			return false, fmt.Errorf("unable to create ssh executor %w", err)
		}
		success, err = w.runCommand(success, ex4)
		if err != nil {
			return false, fmt.Errorf("failed to checkout %w", err)
		}
		//4. run the setup script, if it is provided
		if w.setupScript != "" {
			cmd5 := []string{"sh", w.setupScript}
			w.printAndStreamCommand(cmd5)
			ex5, err := executor.NewNodeSSHExecutor(nd, repoDir, cmd5)
			if err != nil {
				return false, fmt.Errorf("unable to create ssh executor %w", err)
			}
			success, err = w.runCommand(success, ex5)
			if err != nil {
				return false, fmt.Errorf("failed to run setup script")
			}
		}
		//5. Run the run script
		cmd6 := []string{"sh", w.setupScript}
		w.printAndStreamCommand(cmd6)
		ex6, err := executor.NewNodeSSHExecutor(nd, repoDir, cmd6)
		if err != nil {
			return false, fmt.Errorf("unable to create ssh executor %w", err)
		}
		success, err = w.runCommand(success, ex6)
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
		success, err = w.runCommand(success, exd2)
		if err != nil {
			return false, fmt.Errorf("unable to cleanup workdir in ssh node %w", err)
		}
	} else {
		//local executor
		w.printAndStream("running tests locally")
		//1. clone the repo
		cmd1 := []string{"git", "clone", w.repoURL, w.repoDir}
		w.printAndStreamCommand(cmd1)
		ex1 := executor.NewLocalExecutor(cmd1)
		success, err = w.runCommand(true, ex1)
		if err != nil {
			return false, fmt.Errorf("failed to clone repo %w", err)
		}
		//2. change to repodir
		os.Chdir(w.repoDir)
		//3. Fetch if needed
		if w.kind == messages.RequestTypePR {
			chkout = fmt.Sprintf("pr%s", w.target)
			cmd3 := []string{"git", "fetch", "origin", fmt.Sprintf("pull/%s/head:%s", w.target, chkout)}
			w.printAndStreamCommand(cmd3)
			ex3 := executor.NewLocalExecutor(cmd3)
			success, err = w.runCommand(success, ex3)
			if err != nil {
				return false, fmt.Errorf("failed to fetch pr %w", err)
			}
		} else if w.kind == messages.RequestTypeBranch {
			chkout = w.target
		} else if w.kind == messages.RequestTypeTag {
			chkout = fmt.Sprintf("tags/%s", w.target)
		}
		//4 checkout
		cmd4 := []string{"git", "checkout", chkout}
		w.printAndStreamCommand(cmd4)
		ex4 := executor.NewLocalExecutor(cmd4)
		success, err = w.runCommand(success, ex4)
		if err != nil {
			return false, fmt.Errorf("failed to checkout %w", err)
		}
		//5 run the setup script, if it is provided
		if w.setupScript != "" {
			cmd5 := []string{"sh", w.setupScript}
			w.printAndStreamCommand(cmd5)
			ex5 := executor.NewLocalExecutor(cmd5)
			success, err = w.runCommand(success, ex5)
			if err != nil {
				return false, fmt.Errorf("failed to run setup script")
			}
		}
		//6 run run script
		cmd6 := []string{"sh", w.setupScript}
		w.printAndStreamCommand(cmd6)
		ex6 := executor.NewLocalExecutor(cmd6)
		success, err = w.runCommand(success, ex6)
		if err != nil {
			return false, fmt.Errorf("failed to run run script")
		}
		//2C. getout of repodir
		os.Chdir("..")
	}
	return status, nil
}

// 4
func (w *Worker) test() (bool, error) {
	status := true
	var err error

	if w.sshNodes != nil {
		for _, nd := range w.sshNodes.Nodes {
			success, err := w.runTests(&nd)
			if err != nil {
				return false, err
			}
			if status {
				status = success
			}
		}
	} else {
		status, err = w.runTests(nil)
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
	return nil
}

func (w *Worker) Shutdown() error {
	return w.rcvq.Shutdown()
}
