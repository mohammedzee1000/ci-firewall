package worker

import (
	"fmt"
	"github.com/mohammedzee1000/ci-firewall/pkg/executor"
	"github.com/mohammedzee1000/ci-firewall/pkg/messages"
	"github.com/mohammedzee1000/ci-firewall/pkg/node"
	"github.com/mohammedzee1000/ci-firewall/pkg/util"
	"k8s.io/klog/v2"
	"path/filepath"
)

func (w *Worker) setupGit(oldStatus bool, ex executor.Executor, repoDir string) (bool, error) {
	if oldStatus {
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
	//Remove any existing workdir of same name, usually due to termination of jobs
	status, err := w.runCommand(true, ex, "", []string{"rm", "-rf", workDir})
	if err != nil {
		err := w.handleCommandError(ex.GetTags(), err)
		if err != nil {
			return false, err
		}
	}
	//create new workdir and repo directory
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
//if oldSuccess is false, then this is skipped
func (w *Worker) tearDownTests(oldSuccess bool, ex executor.Executor, workDir string) (bool, error) {
	klog.V(2).Infof("tearing down test env")
	if oldSuccess {
		status, err := w.runCommand(oldSuccess, ex, "", []string{"rm", "-rf", workDir})
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
	baseWorkDir := util.GetBaseWorkDir(w.ciMessage.ReceiveQueueName)
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
