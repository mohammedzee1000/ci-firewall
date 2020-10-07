package worker

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"

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

func (w *Worker) runCommand(cmdArgs []string, stream bool) (bool, error) {
	var err error
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

func (w *Worker) runTests(nd *node.Node) (bool, error) {
	var status bool
	var err error
	//inform the user
	runMsg := "Running Tests"
	if nd != nil {
		runMsg = fmt.Sprintf("%s on node %s", runMsg, nd.OS)
	}
	w.printAndStream(runMsg)

	//common ops here
	var chkout string
	var fetchParam string
	if w.kind == messages.RequestTypePR {
		chkout = fmt.Sprintf("pr%s", w.target)
		fetchParam = fmt.Sprintf("origin pull/%s/HEAD:%s", w.target, chkout)
	} else if w.kind == messages.RequestTypeBranch {
		chkout = w.target
	} else if w.kind == messages.RequestTypeTag {
		chkout = fmt.Sprintf("tags/%s", w.target)
	}

	//if we need to run the tests on some node, do this here
	if nd != nil {
		//Setup node
		w.printAndStream("Preparing remote node...")
		cmd1 := fmt.Sprintf("rm -rf %s;mkdir -p %s/{bin,%s}", w.workdir, w.workdir, w.repoDir)
		w.runCommand(nd.NodeSSHCommand(cmd1), false)
		err = nd.GenerateEnvFile(w.envFile, w.envVars)
		if err != nil {
			return false, fmt.Errorf("failed to generate env file %w", err)
		}
		cmd2 := nd.SCPToWorkDirCommand(w.envFile, w.workdir)
		w.runCommand(cmd2, false)
		//Run the tests
		cmd3 := fmt.Sprintf("export BINDIR=%[1]s/bin; cd %[1]s; . %[2]s; git clone %[3]s ./%[4]s; cd %[4]s; git fetch %[5]s; git checkout %[6]s; . %[7]s; . %[8]s", w.workdir, w.envFile, w.repoURL, w.repoDir, fetchParam, chkout, w.setupScript, w.runScript)
		w.printAndStreamCommandString(cmd3)
		w.runCommand(nd.NodeSSHCommand(cmd3), true)
	} else {
		//non node testing happens here (in-place)
		//* set required envs
		w.envVars["BASE_OS"] = "linux"
		w.envVars["ARCH"] = "amd64"
		for k, v := range w.envVars {
			w.printAndStream(fmt.Sprintf("%s=%s", k, v))
			os.Setenv(k, v)
		}
		cmd4 := []string{"git", "clone", w.repoURL, w.repoDir}
		w.printAndStreamCommand(cmd4)
		w.runCommand(cmd4, false)
		os.Chdir(w.repoDir)
		cmd5 := []string{"git", "fetch", fetchParam}
		w.printAndStreamCommand(cmd5)
		w.runCommand(cmd5, false)
		cmd6 := []string{"git", "checkout", chkout}
		w.printAndStreamCommand(cmd6)
		w.runCommand(cmd6, false)
		//tmp
		cmd7 := []string{"sh", w.setupScript}
		w.printAndStreamCommand(cmd7)
		w.runCommand(cmd7, true)
		cmd8 := []string{"sh", w.runScript}
		w.printAndStreamCommand(cmd8)
		w.runCommand(cmd8, true)
	}
	return status, nil
}

// 4
func (w *Worker) testing() (bool, error) {
	status := true
	var err error
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
