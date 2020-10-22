package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/mohammedzee1000/ci-firewall/pkg/executor"
	"github.com/mohammedzee1000/ci-firewall/pkg/node"
)

func runTestsOnNodes(ndpath, runscript, context string) (bool, error) {
	nodes, err := node.NodesFromFile(ndpath)
	if err != nil {
		return false, fmt.Errorf("failed to get nodes %s", err)
	}

	for _, n := range nodes.Nodes {
		crm, err := executor.NewNodeSSHExecutor(&n, "", []string{"rm", "-rf", context})
		if err != nil {
			return false, fmt.Errorf("failed to get node executor %w", err)
		}
		cs, err := runCMD(true, crm)
		if err != nil {
			return false, fmt.Errorf("failed to delete context dir %w", err)
		}
		cmd, err := executor.NewNodeSSHExecutor(&n, "", []string{"mkdir", "-p", context})
		if err != nil {
			return false, fmt.Errorf("failed to get node executor %w", err)
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
			return false, fmt.Errorf("failed to run run script %s", err)
		}
		return cs, nil
	}
	return false, nil
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
	nf := os.Getenv("ND_FILE")
	if nf == "" {
		log.Fatal("please provide node information ND_FILE")
	}
	rs := os.Getenv("RUN_SCRIPT")
	if rs != "" {
		log.Fatalf("please provide run script RUN_SCRIPT")
	}
	ctx := os.Getenv("CONTEXT")
	if ctx == "" {
		log.Fatalf("please provide context CONTEXT")
	}
	success, err := runTestsOnNodes(nf, rs, ctx)
	if err != nil {
		log.Fatalf("failed to run tests %s", err)
	}
	if !success {
		log.Fatalf("tests failed see logs ^")
	}
}
