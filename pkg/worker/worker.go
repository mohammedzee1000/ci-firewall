package worker

import (
	"bufio"
	"fmt"
	"os"

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
	workdir         string
	envVars         map[string]string
	envFile         string
	repoDir         string
	multios         bool
}

func NewWorker(amqpURI, jenkinsURL, jenkinsUser, jenkinsPassword, jenkinsProject, kind, repoURL, target, setupScript, runScript, rcvQueueName, workdir string, envVars map[string]string, jenkinsBuild int, multiOS bool) *Worker {
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
		workdir:         workdir,
		envFile:         "env.sh",
		repoDir:         "repo",
		multios:         multiOS,
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

func (w *Worker) getCommands() [][]string {
	var chkout string
	var cmds [][]string
	//0
	cmds = append(cmds, []string{"rm", "-rf", w.workdir})
	//1
	cmds = append(cmds, []string{"mkdir", w.workdir})
	//2
	cmds = append(cmds, []string{"git", "clone", w.repoURL, w.repoDir})
	if w.kind == messages.RequestTypePR {
		chkout = fmt.Sprintf("pr%s", w.target)
		//potential 3
		cmds = append(cmds, []string{"git", "fetch", "origin", fmt.Sprintf("pull/%s/head:%s", w.target, chkout)})
	} else if w.kind == messages.RequestTypeBranch {
		chkout = w.target
	} else if w.kind == messages.RequestTypeTag {
		chkout = fmt.Sprintf("tags/%s", w.target)
	}
	//3 if 3 done, else 4
	cmds = append(cmds, []string{"git", "checkout", chkout})
	//5
	cmds = append(cmds, []string{".", w.setupScript})
	//5
	cmds = append(cmds, []string{".", w.runScript})
	return cmds
}

func (w *Worker) runTests(nd *node.Node) (bool, error) {
	var status bool
	var success bool
	var err error
	var chkout string

	// If we have node, then we need to use node executor
	if nd != nil {

	} else {
		//local executor
		w.printAndStream("running tests locally")
		//1. make sub workdir
		cmd1 := []string{"mkdir", w.workdir}
		w.printAndStreamCommand(cmd1)
		ex1 := executor.NewLocalExecutor(cmd1)
		success, err = w.runCommand(true, ex1)
		if err != nil {
			return false, fmt.Errorf("failed to create subworkdir %w", err)
		}
		//2. cd into workdir
		w.printAndStream("changing into subworkdir")
		os.Chdir(w.workdir)
		//3. clone the repo
		cmd3 := []string{"git", "clone", w.repoURL, w.repoDir}
		w.printAndStreamCommand(cmd3)
		ex3 := executor.NewLocalExecutor(cmd3)
		success, err = w.runCommand(success, ex3)
		if err != nil {
			return false, fmt.Errorf("failed to clone repo %w", err)
		}
		//4. change to repodi
		os.Chdir(w.repoDir)
		//5. Fetch if needed
		if w.kind == messages.RequestTypePR {
			chkout = fmt.Sprintf("pr%s", w.target)
			cmd5 := []string{"git", "fetch", "origin", fmt.Sprintf("pull/%s/head:%s", w.target, chkout)}
			w.printAndStreamCommand(cmd5)
			ex5 := executor.NewLocalExecutor(cmd5)
			success, err = w.runCommand(success, ex5)
			if err != nil {
				return false, fmt.Errorf("failed to fetch pr %w", err)
			}
		} else if w.kind == messages.RequestTypeBranch {
			chkout = w.target
		} else if w.kind == messages.RequestTypeTag {
			chkout = fmt.Sprintf("tags/%s", w.target)
		}
		//6 checkout
		cmd6 := []string{"git", "checkout", chkout}
		w.printAndStreamCommand(cmd6)
		ex6 := executor.NewLocalExecutor(cmd6)
		success, err = w.runCommand(success, ex6)
		if err != nil {
			return false, fmt.Errorf("failed to checkout %w", err)
		}
		//7 run the setup script
		cmd7 := []string{"sh", w.setupScript}
		w.printAndStreamCommand(cmd7)
		ex7 := executor.NewLocalExecutor(cmd7)
		success, err = w.runCommand(success, ex7)
		if err != nil {
			return false, fmt.Errorf("failed to run setup script")
		}
		//8 run run script
		cmd8 := []string{"sh", w.setupScript}
		w.printAndStreamCommand(cmd8)
		ex8 := executor.NewLocalExecutor(cmd8)
		success, err = w.runCommand(success, ex8)
		if err != nil {
			return false, fmt.Errorf("failed to run run script")
		}
		//4C. getout of repodir
		os.Chdir("..")
		//2C. getout of workdir
		os.Chdir("..")
	}
	return status, nil
}

// 4
func (w *Worker) testing() (bool, error) {
	status := true
	var err error

	if w.multios {
		// w.NodeList, err = node.NodeListFromDir("../test-nodes")
		// if err != nil {
		// 	return false, err
		// }
		// for _, nd := range w.NodeList {
		// 	success, err := w.runTests(&nd)
		// 	if err != nil {
		// 		return false, err
		// 	}
		// 	if status {
		// 		status = success
		// 	}
		// }
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
	success, err := w.testing()
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
