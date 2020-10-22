package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/mohammedzee1000/ci-firewall/pkg/executor"
	"github.com/mohammedzee1000/ci-firewall/pkg/node"
)

func handleExecutorError(err error) {
	fmt.Printf("!!!! failed to get node executor, nvm skipping %s !!!!\n", err)
}

func runTestsOnNodes(ndpath, runscript, context string) (bool, error) {
	nodes, err := node.NodesFromFile(ndpath)
	if err != nil {
		return false, fmt.Errorf("failed to get nodes %s", err)
	}
	overallsuccess := true
	for _, n := range nodes.Nodes {
		fmt.Printf("\n!!!! Executing against node %s !!!!\n", n.Name)
		crm, err := executor.NewNodeSSHExecutor(&n, "", []string{"rm", "-rf", context})
		if err != nil {
			handleExecutorError(err)
			return false, nil
		}
		cs, err := runCMD(true, crm)
		if err != nil {
			return false, fmt.Errorf("failed to delete context dir %w", err)
		}
		cmd, err := executor.NewNodeSSHExecutor(&n, "", []string{"mkdir", "-p", context})
		if err != nil {
			handleExecutorError(err)
			return false, nil
		}
		cs, err = runCMD(cs, cmd)
		if err != nil {
			return false, fmt.Errorf("failed to create context dir %s", err)
		}
		rs, err := executor.NewNodeSSHExecutor(&n, context, []string{"sh", runscript})
		if err != nil {
			return false, fmt.Errorf("failed to create context dir %s", err)
		}
		cs, err = runCMD(cs, rs)
		if err != nil {
			handleExecutorError(err)
			return false, nil
		}
		if overallsuccess {
			overallsuccess = cs
		}
	}
	return overallsuccess, nil
}

func runCMD(oldsucess bool, ex *executor.NodeSSHExecutor) (bool, error) {
	var err error
	defer ex.Close()
	if oldsucess {
		ex.SetEnvs(make(map[string]string))
		done := make(chan error)
		ex.ShortStderrToStdOut()
		r, _ := ex.StdoutPipe()
		scanner := bufio.NewScanner(r)
		go func(done chan error) {
			for scanner.Scan() {
				line := scanner.Text()
				fmt.Println(line)
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
	fmt.Println("skipping as previous command failed")
	return false, nil
}

func main() {
	if len(os.Args) < 4 {
		log.Fatalf("usage: ssh-run-cmd [context] [ndfile] [runscript]")
	}
	contx := os.Args[1]
	ndfile := os.Args[2]
	runscript := os.Args[3]
	success, err := runTestsOnNodes(ndfile, runscript, contx)
	if err != nil {
		log.Fatalf("failed to run tests %s", err)
	}
	if !success {
		log.Fatalf("tests failed see logs ^")
	}
}
