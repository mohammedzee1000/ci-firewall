package main

import (
	"fmt"
	"io"
	"log"

	"github.com/mohammedzee1000/ci-firewall/pkg/executor"
	"github.com/mohammedzee1000/ci-firewall/pkg/node"
)

func main() {
	//for poc and testing purpose, do not commit
	//for poc and testing purpose, do not commit
	n := node.Node{
		Name:        "MacOS",
		User:        "qetest",
		Address:     "10.76.98.57",
		Port:        "22",
		BaseOS:      "macos",
		Arch:        "amd64",
		SSHPassword: "qetest",
	}
	ex, err := executor.NewNodeSSHExecutor(&n, "", []string{"echo", "baseos=$BASE_OS"})
	if err != nil {
		log.Fatalf("unable to get executor %s", err)
	}
	defer ex.Close()
	done := make(chan error)
	ex.SetEnvs(make(map[string]string))

	rdr, err := ex.BufferedReader()
	if err != nil {
		log.Fatalf("failed to get buffered reader %s", err)
	}
	go func(done chan error) {
		for {
			data, err := rdr.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					done <- fmt.Errorf("error while reading from buffer %w", err)
				}
				break
			}
			fmt.Println(data)
		}
		done <- nil
	}(done)
	err = ex.Start()
	if err != nil {
		log.Fatalf("failed to run command %s", err)
	}
	err = <-done
	if err != nil {
		log.Fatalf("err %s", err)
	}
	ex.Wait()

	// ex.ShortStderrToStdOut()
	// r, _ := ex.StdoutPipe()
	// scanner := bufio.NewScanner(r)
	// go func(done chan error) {
	// 	for scanner.Scan() {
	// 		txt := scanner.Text()
	// 		fmt.Print(txt)
	// 	}
	// 	done <- nil
	// }(done)
	// ex.Start()
	// ex.Wait()
	// fmt.Println("doing a combined output")
	// ex1, err := executor.NewNodeSSHExecutor(&n, "", []string{"echo", "baseos=$BASE_OS"})
	// defer ex1.Close()
	// ex1.SetEnvs(make(map[string]string))
	// o, err := ex1.CombinedOutput()
	// if err != nil {
	// 	log.Fatalf("failed to execute combined output %s", err)
	// }
	// fmt.Sprintln(string(o))
}
