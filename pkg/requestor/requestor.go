package requestor

import (
	"encoding/json"
	"fmt"

	"github.com/mohammedzee1000/ci-firewall/pkg/messages"
	"github.com/mohammedzee1000/ci-firewall/pkg/queue"
	"github.com/streadway/amqp"
)

type Requestor struct {
	sendq            *queue.JMSAMQPQueue
	rcvq             *queue.AMQPQueue
	jenkinsBuild     int
	repoURL          string
	kind             string
	target           string
	runscript        string
	setupScript      string
	recieveQueueName string
	runScriptURL     string
	mainBranch       string
	done             chan error
}

func NewRequestor(amqpURI, sendqName, exchangeName, topic, repoURL, kind, target, setupScript, runscript, recieveQueueName, runScriptURL, mainBranch string) *Requestor {
	r := &Requestor{
		sendq:            queue.NewJMSAMQPQueue(amqpURI, sendqName, exchangeName, topic),
		rcvq:             queue.NewAMQPQueue(amqpURI, recieveQueueName),
		jenkinsBuild:     -1,
		repoURL:          repoURL,
		kind:             kind,
		target:           target,
		runscript:        runscript,
		recieveQueueName: recieveQueueName,
		setupScript:      setupScript,
		runScriptURL:     runScriptURL,
		mainBranch:       mainBranch,
		done:             make(chan error),
	}
	return r
}

func (r *Requestor) initQueus() error {
	if r.kind != messages.RequestTypePR && r.kind != messages.RequestTypeBranch && r.kind != messages.RequestTypeTag {
		return fmt.Errorf("kind should be %s, %s or %s", messages.RequestTypePR, messages.RequestTypeBranch, messages.RequestTypeTag)
	}
	err := r.sendq.Init()
	if err != nil {
		return fmt.Errorf("failed to initalize send q %w", err)
	}
	err = r.rcvq.Init()
	if err != nil {
		return fmt.Errorf("failed to initialize rcvq %w", err)
	}
	return nil
}

func (r *Requestor) sendBuildRequest() error {
	var err error
	err = r.sendq.Publish(messages.NewRemoteBuildRequestMessage(r.repoURL, r.kind, r.target, r.setupScript, r.runscript, r.recieveQueueName, r.runScriptURL, r.mainBranch))
	if err != nil {
		return fmt.Errorf("failed to send build request %w", err)
	}
	return nil
}

func (r *Requestor) consumeMessages() error {
	err := r.rcvq.Consume(func(deliveries <-chan amqp.Delivery, done chan error) {
		success := true
		for d := range deliveries {
			m := &messages.Message{}
			err1 := json.Unmarshal(d.Body, m)
			if err1 != nil {
				done <- fmt.Errorf("failed to unmarshal as message %w", err1)
				return
			}
			if r.jenkinsBuild == -1 && m.IsBuild() {
				bm := messages.NewBuildMessage(-1)
				err1 = json.Unmarshal(d.Body, bm)
				if err1 != nil {
					done <- fmt.Errorf("failed to unmarshal as build message %w", err1)
					return
				}
				r.jenkinsBuild = bm.Build
				fmt.Printf("Following jenkins build %d\n", r.jenkinsBuild)
			} else if r.jenkinsBuild == m.Build {
				if m.ISLog() {
					lm := messages.NewLogsMessage(-1, "")
					err1 = json.Unmarshal(d.Body, lm)
					if err1 != nil {
						done <- fmt.Errorf("failed to unmarshal as logs message %w", err1)
						return
					}
					fmt.Println(lm.Logs)
				} else if m.IsStatus() {
					sm := messages.NewStatusMessage(-1, false)
					err1 = json.Unmarshal(d.Body, sm)
					if err1 != nil {
						done <- fmt.Errorf("failed to unmarshal as status message %w", err1)
					}
					if success {
						success = sm.Success
					}
				} else if m.IsFinal() {
					if success {
						done <- nil
					} else {
						done <- fmt.Errorf("Failed the test, see logs above ^")
					}
					return
				}
			}
			d.Ack(false)
		}
	}, r.done)
	if err != nil {
		return err
	}
	return nil
}

func (r *Requestor) Run() error {
	err := r.initQueus()
	if err != nil {
		return err
	}
	err = r.sendBuildRequest()
	if err != nil {
		return err
	}
	err = r.consumeMessages()
	if err != nil {
		return err
	}
	return nil
}

func (r *Requestor) Done() chan error {
	return r.done
}

func (r *Requestor) ShutDown() error {
	err := r.sendq.Shutdown()
	if err != nil {
		return fmt.Errorf("failed to shutdown send q %w", err)
	}
	err = r.rcvq.Shutdown(true)
	if err != nil {
		return fmt.Errorf("failed to shutdown rcv q %w", err)
	}
	return nil
}
