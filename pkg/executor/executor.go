package executor

import (
	"bufio"
)

type Executor interface {
	InitCommand(string, []string, map[string]string) (*bufio.Reader, error)
	Start() error
	Wait() (bool, error)
	Close() error
}
