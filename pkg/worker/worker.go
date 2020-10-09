package worker

import (
	"bufio"
	"fmt"

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
	NodeList        node.NodeList
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

// func (w *Worker) fetchRepo() (bool, error) {
// 	//TODO: handle printAndStream errors
// 	var chkout string
// 	//Get the repo
// 	wd, err := os.Getwd()
// 	if err != nil {
// 		return false, fmt.Errorf("unable to get wd %w", err)
// 	}
// 	w.printAndStream("Getting repo...")
// 	cmd1 := []string{"git", "clone", w.repoURL, filepath.Join(wd, w.workdir, w.repoDir)}
// 	w.printAndStreamCommand(cmd1)
// 	s1, err := w.runCommand(true, cmd1, false)
// 	if err != nil {
// 		return false, fmt.Errorf("failed to clone %w", err)
// 	}
// 	err = os.Chdir(w.repoDir)
// 	if err != nil {
// 		return false, fmt.Errorf("failed to switch to repodir %w", err)
// 	}
// 	if w.kind == messages.RequestTypePR {
// 		chkout = fmt.Sprintf("pr%s", w.target)
// 		cmd2 := []string{"git", "fetch", "origin", fmt.Sprintf("pull/%s/head:%s", w.target, chkout)}
// 		w.printAndStreamCommand(cmd2)
// 		s1, err = w.runCommand(s1, cmd2, false)
// 		if err != nil {
// 			return false, fmt.Errorf("failed to fetch PR %w", err)
// 		}
// 	} else if w.kind == messages.RequestTypeBranch {
// 		chkout = w.target
// 	} else if w.kind == messages.RequestTypeTag {
// 		chkout = fmt.Sprintf("tags/%s", w.target)
// 	} else {
// 		return false, fmt.Errorf("unknown kind")
// 	}
// 	cmd3 := []string{"git", "checkout", chkout}
// 	w.printAndStreamCommand(cmd3)
// 	s3, err := w.runCommand(s1, cmd3, false)
// 	if err != nil {
// 		return false, fmt.Errorf("failed to checkout %s, %w", chkout, err)
// 	}
// 	err = os.Chdir(wd)
// 	if err != nil {
// 		return false, fmt.Errorf("failed to switch back %w", err)
// 	}
// 	return s3, nil
// }

func (w *Worker) getCommands() [][]string {
	var chkout string
	var cmds [][]string
	cmds = append(cmds, []string{"git", "clone", w.repoDir, w.workdir})
	if w.kind == messages.RequestTypePR {
		chkout = fmt.Sprintf("pr%s", w.target)
		cmds = append(cmds, []string{"git", "fetch", "origin", fmt.Sprintf("pull/%s/head:%s", w.target, chkout)})
	} else if w.kind == messages.RequestTypeBranch {
		chkout = w.target
	} else if w.kind == messages.RequestTypeTag {
		chkout = fmt.Sprintf("tags/%s", w.target)
	}
	cmds = append(cmds, []string{"git", "checkout", chkout})
	cmds = append(cmds, []string{"sh", w.setupScript})
	cmds = append(cmds, []string{"sh", w.runScript})
	return cmds
}

func (w *Worker) runTests(nd *node.Node) (bool, error) {
	var status bool
	var success bool
	var err error
	//get the command list to run
	cmds := w.getCommands()
	// If we have node, then we need to use node executor
	if nd != nil {
		w.printAndStream(fmt.Sprintf("running tests on node %s", nd.Name))
		ex1, err := executor.NewNodeExecutor(nd, cmds[0])
		if err != nil {
			return false, fmt.Errorf("unable to initialize executor for command %v on node %s %w", cmds[0], nd.Name, err)
		}
		ex1.SetEnvs(w.envVars)
		success, err = w.runCommand(true, ex1)
		if err != nil {
			return false, fmt.Errorf("failed to execute command %v", cmds[0])
		}
		for _, cmd := range cmds {
			ex, err := executor.NewNodeExecutor(nd, cmd)
			if err != nil {
				return false, fmt.Errorf("unable to initialize executor for command %v on node %s %w ", cmd, nd.Name, err)
			}
			ex.SetEnvs(w.envVars)
			success, err = w.runCommand(success, ex)
			if err != nil {
				return false, fmt.Errorf("failed to execute command %v", cmds[0])
			}
		}
	} else {
		w.printAndStream("running tests locally")
		ex1 := executor.NewLocalExecutor(cmds[0])
		ex1.SetEnvs(w.envVars)
		success, err = w.runCommand(true, ex1)
		if err != nil {
			return false, fmt.Errorf("failed to execute command %v", cmds[0])
		}
		for _, cmd := range cmds[1:] {
			ex := executor.NewLocalExecutor(cmd)
			ex.SetEnvs(w.envVars)
			success, err = w.runCommand(success, executor.NewLocalExecutor(cmd))
			if err != nil {
				return false, fmt.Errorf("failed to run command %v", cmd)
			}
		}
	}
	return status, nil
}

// 4
func (w *Worker) testing() (bool, error) {
	status := true
	var err error
	// s1, err := w.fetchRepo()
	// if err != nil {
	// 	return false, fmt.Errorf("unable to fetch repo %w", err)
	// }
	// if s1 {
	if w.multios {
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
