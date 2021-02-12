package printstreambuffer

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/acarl005/stripansi"
	"github.com/mohammedzee1000/ci-firewall/pkg/messages"
	"github.com/mohammedzee1000/ci-firewall/pkg/queue"
)

type PrintStreamBuffer struct {
	q          *queue.AMQPQueue
	message    string
	bufferSize int
	counter    int
	buildno    int
	envs       map[string]string
	redact     bool
}

func NewPrintStreamBuffer(q *queue.AMQPQueue, bufsize int, buildno int, envs map[string]string, redact bool) *PrintStreamBuffer {
	return &PrintStreamBuffer{
		q:          q,
		bufferSize: bufsize,
		counter:    0,
		buildno:    buildno,
		envs:       envs,
		redact:     redact,
	}
}

func (psb *PrintStreamBuffer) FlushToQueue() error {
	if psb.counter > 0 {
		if psb.q != nil {
			if psb.redact {
				for k, v := range psb.envs {
					if k != "CI" {
						psb.message = strings.ReplaceAll(psb.message, v, fmt.Sprintf("-ENV:%s:REDACTED-", k))
					}
				}
				ipre := regexp.MustCompile(`(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}`)
				psb.message = ipre.ReplaceAllString(psb.message, "-IP REDACTED-")
			}
			lm := messages.NewLogsMessage(psb.buildno, psb.message)
			err := psb.q.Publish(false, lm)
			if err != nil {
				return fmt.Errorf("failed to publish buffer to %w", err)
			}
		}
		psb.message = ""
		psb.counter = 0
	}
	return nil
}

func (psb *PrintStreamBuffer) Print(data string, flushnow bool, stripAnsiColorJenkins bool) error {
	psb.message = fmt.Sprintf("%s%s", psb.message, data)
	psb.counter++
	if stripAnsiColorJenkins {
		fmt.Println(stripansi.Strip(data))
	} else {
		fmt.Println(data)
	}
	if flushnow || psb.counter >= psb.bufferSize {
		return psb.FlushToQueue()
	}
	return nil
}
