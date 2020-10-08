package worker

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
	NodeList        node.NodeList
	multiNode       bool
}

func NewWorker(amqpURI, jenkinsURL, jenkinsUser, jenkinsPassword, jenkinsProject, kind, repoURL, target, setupScript, runScript, rcvQueueName, workdir string, envVars map[string]string, jenkinsBuild int, multiNode bool) *Worker {
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
		multiNode:       multiNode,
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
	// lm := messages.NewLogsMessage(w.jenkinsBuild, msg)
	// err := w.rcvq.Publish(
	// 	false, lm,
	// )
	// if err != nil {
	// 	return fmt.Errorf("failed to stream log message %w", err)
	// }
	return nil
}

func (w *Worker) printAndStreamCommand(cmdArgs []string) error {
	return w.printAndStream(fmt.Sprintf("Executing command %v", cmdArgs))
}

func (w *Worker) printAndStreamCommandString(cmdArgs string) error {
	return w.printAndStream(fmt.Sprintf("Executing command [%s]", cmdArgs))
}

func (w *Worker) runCommand(oldsuccess bool, cmdArgs []string, stream bool) (bool, error) {
	var err error
	if oldsuccess {
		//get jenkins user info
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		if stream {
			done := make(chan error)
			r, _ := cmd.StdoutPipe()
			cmd.Stderr = cmd.Stdout
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
			err = cmd.Start()
			if err != nil {
				return false, fmt.Errorf("failed to run command %w", err)
			}
			err = <-done
			if err != nil {
				return false, err
			}
			if cmd.ProcessState.ExitCode() != 0 {
				return false, nil
			}
			return true, nil
		} else {
			out, err := cmd.CombinedOutput()
			if err != nil {
				return false, fmt.Errorf("failed to execute command %w", err)
			}
			err1 := w.printAndStream(string(out))
			if err1 != nil {
				return false, err1
			}
		}
		return true, nil
	}
	w.printAndStream("previous command failed or skipped, skipping")
	return false, nil
}

func (w *Worker) fetchRepo() (bool, error) {
	//TODO: handle printAndStream errors
	var chkout string
	//Get the repo
	wd, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("unable to get wd %w", err)
	}
	w.printAndStream("Getting repo...")
	cmd1 := []string{"git", "clone", w.repoURL, filepath.Join(wd, w.workdir, w.repoDir)}
	w.printAndStreamCommand(cmd1)
	s1, err := w.runCommand(true, cmd1, false)
	if err != nil {
		return false, fmt.Errorf("failed to clone %w", err)
	}
	err = os.Chdir(w.repoDir)
	if err != nil {
		return false, fmt.Errorf("failed to switch to repodir %w", err)
	}
	if w.kind == messages.RequestTypePR {
		chkout = fmt.Sprintf("pr%s", w.target)
		cmd2 := []string{"git", "fetch", "origin", fmt.Sprintf("pull/%s/head:%s", w.target, chkout)}
		w.printAndStreamCommand(cmd2)
		s1, err = w.runCommand(s1, cmd2, false)
		if err != nil {
			return false, fmt.Errorf("failed to fetch PR %w", err)
		}
	} else if w.kind == messages.RequestTypeBranch {
		chkout = w.target
	} else if w.kind == messages.RequestTypeTag {
		chkout = fmt.Sprintf("tags/%s", w.target)
	} else {
		return false, fmt.Errorf("unknown kind")
	}
	cmd3 := []string{"git", "checkout", chkout}
	w.printAndStreamCommand(cmd3)
	s3, err := w.runCommand(s1, cmd3, false)
	if err != nil {
		return false, fmt.Errorf("failed to checkout %s, %w", chkout, err)
	}
	err = os.Chdir(wd)
	if err != nil {
		return false, fmt.Errorf("failed to switch back %w", err)
	}
	return s3, nil
}

func (w *Worker) runTests(nd *node.Node) (bool, error) {
	var status bool
	//if we need to run the tests on some node, do this here
	if nd != nil {
		//Setup node
		return false, fmt.Errorf("not implemented")
	} else {
		//non multios testing happens here (in-place)
		//* set required envs
		w.envVars["BASE_OS"] = "linux"
		w.envVars["ARCH"] = "amd64"
		for k, v := range w.envVars {
			fmt.Printf("%s=%s", k, v)
			os.Setenv(k, v)
		}
		os.Chdir(w.repoDir)
		cmdsetup := []string{"sh", w.setupScript}
		w.printAndStreamCommand(cmdsetup)
		ssetup, err := w.runCommand(true, cmdsetup, true)
		if err != nil {
			return false, fmt.Errorf("failed to run setup script %w", err)

		}
		cmdrun := []string{"sh", w.runScript}
		w.printAndStreamCommand(cmdrun)
		srun, err := w.runCommand(ssetup, cmdrun, true)
		if err != nil {
			return false, fmt.Errorf("failed to run run script %w", err)
		}
		status = srun
	}
	return status, nil
}

// 4
func (w *Worker) testing() (bool, error) {
	status := true
	var err error
	s1, err := w.fetchRepo()
	if err != nil {
		return false, fmt.Errorf("unable to fetch repo %w", err)
	}
	if s1 {
		if w.multiNode {
			w.NodeList, err = node.NodeListFromDir("../test-nodes")
			if err != nil {
				return false, err
			}
			for _, nd := range w.NodeList {
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
	}
	status = false
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
