package requestor

import (
	"encoding/json"
	"fmt"
	"k8s.io/klog/v2"

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
	jenkinsProject   string
}

func NewRequestor(amqpURI, sendqName, exchangeName, topic, repoURL, kind, target, setupScript, runscript, recieveQueueName, runScriptURL, mainBranch, jenkinsJobName string) *Requestor {
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
		jenkinsProject:   jenkinsJobName,
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
	rbr := messages.NewRemoteBuildRequestMessage(r.repoURL, r.kind, r.target, r.setupScript, r.runscript, r.recieveQueueName, r.runScriptURL, r.mainBranch, r.jenkinsProject)
	klog.V(2).Infof("sending remote build request")
	klog.V(4).Infof("remote build request: %#v", rbr)
	err = r.sendq.Publish(rbr)
	if err != nil {
		return fmt.Errorf("failed to send build request %w", err)
	}
	return nil
}

func (r *Requestor) consumeMessages() error {
	klog.V(2).Infof("listening on rcv queue %s for messages from worker")
	err := r.rcvq.Consume(func(deliveries <-chan amqp.Delivery, done chan error) {
		success := true
		for d := range deliveries {
			klog.V(2).Infof("received message from worker")
			klog.V(4).Infof("Raw message %#v", d.Body)
			m := &messages.Message{}
			err1 := json.Unmarshal(d.Body, m)
			if err1 != nil {
				done <- fmt.Errorf("failed to unmarshal as message %w", err1)
				return
			}
			//process message only if the jenkins projects match
			if m.JenkinsProject == r.jenkinsProject {
				if m.Build > r.jenkinsBuild {
					//process build or cancel message, only if the message build > current jenkins build
					if m.IsBuild() {
						klog.V(2).Infof("received build message")
						bm := messages.NewBuildMessage(-1,"")
						err1 = json.Unmarshal(d.Body, bm)
						klog.V(4).Infof("received build message %#v", bm)
						if err1 != nil {
							done <- fmt.Errorf("failed to unmarshal as build message %w", err1)
							return
						}
						r.jenkinsBuild = bm.Build
						fmt.Printf("Following jenkins build %d\n", r.jenkinsBuild)
					} else if m.IsCancel() {
						klog.V(2).Infof("received cancel message from newer build")
						cm := messages.NewCancelMessage(-1, "")
						err1 := json.Unmarshal(d.Body, cm)
						if err1 != nil {
							done <- fmt.Errorf("failed to unmarshal cancel message %w", err1)
						}
						fmt.Printf("\n!!! Detected newer build %d and following it. Please ignore logs above this point !!!\n", cm.Build)
						r.jenkinsBuild = -1
						success = true
					}
				} else if r.jenkinsBuild == m.Build {
					//process other types of messages, only if message build matches current jenkins build
					if m.ISLog() {
						klog.V(2).Infof("received log message")
						lm := messages.NewLogsMessage(-1, "", "")
						err1 = json.Unmarshal(d.Body, lm)
						if err1 != nil {
							done <- fmt.Errorf("failed to unmarshal as logs message %w", err1)
							return
						}
						klog.V(4).Infof("log message %#v", lm)
						fmt.Println(lm.Logs)
					} else if m.IsStatus() {
						klog.V(2).Infof("received status message")
						sm := messages.NewStatusMessage(-1, false, "")
						err1 = json.Unmarshal(d.Body, sm)
						if err1 != nil {
							done <- fmt.Errorf("failed to unmarshal as status message %w", err1)
						}
						klog.V(4).Infof("status message %#v", sm)
						if success {
							klog.V(2).Infof("Updating success status")
							success = sm.Success
						} else {
							klog.V(2).Infof("Skipping update to status as its already false")
						}
					} else if m.IsFinal() {
						klog.V(2).Infof("received final message")
						if success {
							done <- nil
						} else {
							done <- fmt.Errorf("Failed the test, see logs above ^")
						}
						return
					} else {
						klog.V(2).Infof("skipping message as message build is lesser than currently followed build")
					}
				}
			} else {
				klog.V(2).Infof("skipping message as job name of message did not match job name expected by requester")
				klog.V(4).Infof("want %s, got %s", r.jenkinsProject, m.JenkinsProject)
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
