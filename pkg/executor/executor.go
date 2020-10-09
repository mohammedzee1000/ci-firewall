package executor

import "io"

type Executor interface {
	StdoutPipe() (io.ReadCloser, error)
	ShortStderrToStdOut()
	Start() error
	Wait() error
	ExitCode() int
	Close() error
	SetEnvs(map[string]string) error
}
