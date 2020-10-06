package worker

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"

	"github.com/mohammedzee1000/ci-firewall/pkg/jenkins"
	"github.com/mohammedzee1000/ci-firewall/pkg/messages"
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
	runScript       string
	workdir         string
	multiNode       bool
}

func NewWorker(amqpURI, jenkinsURL, jenkinsUser, jenkinsPassword, jenkinsProject, kind, repoURL, target, runScript, rcvQueueName, workdir string, jenkinsBuild int, multiNode bool) *Worker {
	w := &Worker{
		rcvq:            queue.NewAMQPQueue(amqpURI, rcvQueueName),
		kind:            kind,
		repoURL:         repoURL,
		target:          target,
		runScript:       runScript,
		jenkinsProject:  jenkinsProject,
		jenkinsBuild:    jenkinsBuild,
		jenkinsURL:      jenkinsURL,
		jenkinsUser:     jenkinsUser,
		jenkinsPassword: jenkinsPassword,
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

func (w *Worker) runCmd(cmd *exec.Cmd, stream bool) (bool, error) {
	var err error
	cmdMsg := fmt.Sprintf("Executing command %v", cmd.Args)
	fmt.Println(cmdMsg)
	lmc := messages.NewLogsMessage(w.jenkinsBuild, cmdMsg)
	err = w.rcvq.Publish(false, lmc)
	if err != nil {
		return false, fmt.Errorf("unable to send log message %w", err)
	}
	if stream {
		done := make(chan error)
		r, _ := cmd.StdoutPipe()
		cmd.Stderr = cmd.Stdout
		scanner := bufio.NewScanner(r)
		go func(done chan error) {
			for scanner.Scan() {
				line := scanner.Text()
				fmt.Println(line)
				lm := messages.NewLogsMessage(w.jenkinsBuild, line)
				err1 := w.rcvq.Publish(
					false, lm,
				)
				if err1 != nil {
					done <- fmt.Errorf("unable to send log message %w", err1)
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
	} else {
		out, err := cmd.CombinedOutput()
		if err != nil {
			return false, fmt.Errorf("failed to execute command %w", err)
		}
		fmt.Println(string(out))
		lm := messages.NewLogsMessage(w.jenkinsBuild, string(out))
		err1 := w.rcvq.Publish(
			false, lm,
		)
		if err1 != nil {
			return false, fmt.Errorf("unable to send log message %w", err1)
		}
	}
	return true, nil
}

// 4
func (w *Worker) runTests() (bool, error) {
	var status bool
	if w.multiNode {
		return false, fmt.Errorf("Not Implemented yet")
	} else {
		w.workdir = "./odo"
		s1, err := w.runCmd(exec.Command("git", "clone", w.repoURL, w.workdir), false)
		if err != nil {
			return false, err
		}
		os.Chdir(w.workdir)
		if !s1 {
			return false, nil
		}
		chkout := ""
		if w.kind == messages.RequestTypePR {
			chkout = fmt.Sprintf("pr%s", w.target)
			fetch := fmt.Sprintf("pull/%s/head:%s", w.target, chkout)
			s2, err := w.runCmd(exec.Command("git", "fetch", "origin", fetch), false)
			if err != nil {
				return false, err
			}
			if !s2 {
				return false, nil
			}
		}
		s3, err := w.runCmd(exec.Command("git", "checkout", chkout), false)
		if err != nil {
			return false, err
		}
		if !s3 {
			return false, nil
		}
		s4, err := w.runCmd(exec.Command("sh", w.runScript), true)
		if err != nil {
			return false, fmt.Errorf("failed to execute run script, %w", err)
		}
		if s4 {
			s4 = false
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
	success, err := w.runTests()
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
