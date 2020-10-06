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

func NewWorker(amqpURI, jenkinsURL, jenkinsUser, jenkinsPassword, jenkinsProject, kind, repoURL, target, runScript, rcvQueueName string, jenkinsBuild int, multiNode bool) *Worker {
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

func (w *Worker) runCmd(cmd *exec.Cmd) (bool, error) {
	var err error
	done := make(chan error)
	fmt.Println(cmd.Args)
	r, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	scanner := bufio.NewScanner(r)
	go func(done chan error) {
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
			// lm := messages.NewLogsMessage(w.jenkinsBuild, line)
			// err1 := w.rcvq.Publish(
			// 	false, lm,
			// )
			// if err1 != nil {
			// 	done <- fmt.Errorf("unable to send log message %w", err)
			// }
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
	fmt.Println("exit code ", cmd.ProcessState.ExitCode())
	if cmd.ProcessState.ExitCode() != 0 {
		return false, nil
	}
	return true, nil
}

func (w *Worker) runNoStream(cmd *exec.Cmd) (bool, error) {
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}
	fmt.Println(out)
	return true, nil
}

// 4
func (w *Worker) runTests() (bool, error) {
	if w.multiNode {
		return false, fmt.Errorf("Not Implemented yet")
	} else {
		w.workdir = "./odo"
		s1, err := w.runNoStream(exec.Command("git", "clone", w.repoURL, w.workdir))
		if err != nil {
			return false, err
		}
		os.Chdir(w.workdir)
		if !s1 {
			fmt.Println("Clone failed")
			return false, nil
		}
		chk := ""
		if w.kind == messages.RequestTypePR {
			fmt.Println("checking out pr yay!!")
			s2, err := w.runCmd(exec.Command("git", "fetch", "origin", fmt.Sprintf("pull/%s/head:pr%s", w.target, w.target)))
			if err != nil {
				return false, err
			}
			chk = fmt.Sprintf("pr%s", w.target)
			if !s2 {
				return false, nil
			}
		}
		s3, err := w.runCmd(exec.Command("git", "checkout", chk))
		if err != nil {
			return false, err
		}
		if !s3 {
			return false, nil
		}
		_, err = w.runCmd(exec.Command("make", "test"))
	}
	return false, nil
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
	// if err := w.sendBuildInfo(); err != nil {
	// 	return fmt.Errorf("failed to send build info %w", err)
	// }
	success, err := w.runTests()
	if err != nil {
		return fmt.Errorf("failed to run tests %w", err)
	}
	fmt.Printf("Success : %t", success)
	// if err := w.sendStatusMessage(success); err != nil {
	// 	return fmt.Errorf("failed to send status message %w", err)
	// }
	return nil
}

func (w *Worker) Shutdown() error {
	return w.rcvq.Shutdown()
}
