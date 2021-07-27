package printstreambuffer

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/acarl005/stripansi"
	"github.com/mohammedzee1000/ci-firewall/pkg/messages"
	"github.com/mohammedzee1000/ci-firewall/pkg/queue"
)

func applyRedactions(originalMsg string, redactIP bool, redactENVs map[string]string, exceptions []string) string {
	var newMsg string
	newMsg = originalMsg
	ipre := regexp.MustCompile(`(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}`)
	if redactIP {
		ipre.ReplaceAllString(newMsg, "REDACTED:IP")
	}
	for k, v := range redactENVs {
		exception := false
		for _, it := range exceptions {
			if strings.Contains(v, it) {
				exception = true
				break
			}
		}
		if !exception {
			newMsg = strings.ReplaceAll(newMsg, v, fmt.Sprintf("REDACTED:ENV:%s", k))
		}
	}
	return newMsg
}

type PrintStreamBuffer struct {
	q              *queue.AMQPQueue
	message        string
	bufferSize     int
	counter        int
	buildno        int
	exceptions     []string
	envs           map[string]string
	redact         bool
	jenkinsProject string
}

func NewPrintStreamBuffer(q *queue.AMQPQueue, bufsize int, buildno int, jenkinsProject string, envs map[string]string, redact bool, exceptions []string) *PrintStreamBuffer {
	return &PrintStreamBuffer{
		q:              q,
		bufferSize:     bufsize,
		counter:        0,
		buildno:        buildno,
		envs:           envs,
		exceptions:     exceptions,
		redact:         redact,
		jenkinsProject: jenkinsProject,
	}
}

func (psb *PrintStreamBuffer) FlushToQueue() error {
	if psb.counter > 0 {
		if psb.q != nil {
			msgToSend := psb.message
			if psb.redact {
				msgToSend = applyRedactions(psb.message, true, psb.envs, psb.exceptions)
			}
			lm := messages.NewLogsMessage(psb.buildno, msgToSend, psb.jenkinsProject)
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
