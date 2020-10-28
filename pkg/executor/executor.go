package executor

import (
	"bufio"
)

type Executor interface {
	BufferedReader() (*bufio.Reader, error)
	Start() error
	Wait() error
	ExitCode() int
	Close() error
	SetEnvs(map[string]string, string) error
}
