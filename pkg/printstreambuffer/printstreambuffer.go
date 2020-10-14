package printstreambuffer

import (
	"fmt"

	"github.com/mohammedzee1000/ci-firewall/pkg/messages"
	"github.com/mohammedzee1000/ci-firewall/pkg/queue"
)

type PrintStreamBuffer struct {
	q          *queue.AMQPQueue
	message    string
	bufferSize int
	counter    int
	buildno    int
}

func NewPrintStreamBuffer(q *queue.AMQPQueue, bufsize int, buildno int) *PrintStreamBuffer {
	return &PrintStreamBuffer{
		q:          q,
		bufferSize: bufsize,
		counter:    0,
		buildno:    buildno,
	}
}

func (psb *PrintStreamBuffer) Flush() error {
	if psb.counter > 0 {
		fmt.Println(psb.message)
		lm := messages.NewLogsMessage(psb.buildno, psb.message)
		err := psb.q.Publish(false, lm)
		if err != nil {
			return fmt.Errorf("failed to publish buffer to %w", err)
		}
		psb.message = ""
		psb.counter = 0
	}
	return nil
}

func (psb *PrintStreamBuffer) Println(msg string, immediate bool) error {
	psb.message = fmt.Sprintf("%s\n%s", psb.message, msg)
	psb.counter++
	if immediate || psb.counter >= psb.bufferSize {
		return psb.Flush()
	}
	return nil
}
