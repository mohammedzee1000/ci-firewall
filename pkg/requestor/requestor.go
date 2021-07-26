package requestor

import (
	"encoding/json"
	"fmt"
	"k8s.io/klog/v2"
	"log"
	"time"

	"github.com/mohammedzee1000/ci-firewall/pkg/messages"
	"github.com/mohammedzee1000/ci-firewall/pkg/queue"
	"github.com/streadway/amqp"
)

type Requester struct {
	sendQueue        *queue.JMSAMQPQueue
	receiveQueue     *queue.AMQPQueue
	jenkinsBuild     int
	repoURL          string
	kind             messages.RequestType
	target           string
	runscript        string
	setupScript      string
	receiveQueueName string
	runScriptURL     string
	mainBranch       string
	done             chan error
	jenkinsProject   string
	timeout          time.Duration
}

type NewRequesterOptions struct {
	AMQPURI          string
	SendQueueName    string
	ExchangeName     string
	Topic            string
	RepoURL          string
	Kind             messages.RequestType
	Target           string
	SetupScript      string
	RunScript        string
	ReceiveQueueName string
	RunScriptURL     string
	MainBranch       string
	JenkinsProject   string
	Timeout          time.Duration
}

func NewRequester(nro *NewRequesterOptions) *Requester {
	r := &Requester{
		sendQueue:        queue.NewJMSAMQPQueue(nro.AMQPURI, nro.SendQueueName, nro.ExchangeName, nro.Topic),
		receiveQueue:     queue.NewAMQPQueue(nro.AMQPURI, nro.ReceiveQueueName),
		jenkinsBuild:     -1,
		repoURL:          nro.RepoURL,
		kind:             nro.Kind,
		target:           nro.Target,
		runscript:        nro.RunScript,
		receiveQueueName: nro.ReceiveQueueName,
		setupScript:      nro.SetupScript,
		runScriptURL:     nro.RunScriptURL,
		mainBranch:       nro.MainBranch,
		done:             make(chan error),
		jenkinsProject:   nro.JenkinsProject,
		timeout:          nro.Timeout,
	}
	return r
}

func (r *Requester) initQueues() error {
	k := r.kind
	if k != messages.RequestTypePR && k != messages.RequestTypeBranch && k != messages.RequestTypeTag {
		return fmt.Errorf("kind should be %s, %s or %s", messages.RequestTypePR, messages.RequestTypeBranch, messages.RequestTypeTag)
	}
	err := r.sendQueue.Init()
	if err != nil {
		return fmt.Errorf("failed to initalize send q %w", err)
	}
	err = r.receiveQueue.Init()
	if err != nil {
		return fmt.Errorf("failed to initialize receiveQueue %w", err)
	}
	return nil
}

func (r *Requester) sendBuildRequest() error {
	var err error
	rbr := messages.NewRemoteBuildRequestMessage(r.repoURL, r.kind, r.target, r.setupScript, r.runscript, r.receiveQueueName, r.runScriptURL, r.mainBranch, r.jenkinsProject)
	klog.V(2).Infof("sending remote build request")
	klog.V(4).Infof("remote build request: %#v", rbr)
	err = r.sendQueue.Publish(rbr)
	if err != nil {
		return fmt.Errorf("failed to send build request %w", err)
	}
	return nil
}

func (r *Requester) consumeReplies() error {
	klog.V(2).Infof("listening on rcv queue %s for messages from worker")
	err := r.receiveQueue.Consume(func(deliveries <-chan amqp.Delivery, done chan error) {
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
						bm := messages.NewBuildMessage(-1, "")
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
					if m.IsLog() {
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
							done <- fmt.Errorf("failed the test, see logs above ^")
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
			err := d.Ack(false)
			if err != nil {
				done <- fmt.Errorf("unable to ack msg %w", err)
			}
		}
	}, r.done)
	if err != nil {
		return err
	}
	return nil
}

func (r *Requester) Run() error {
	err := r.initQueues()
	if err != nil {
		return err
	}
	err = r.sendBuildRequest()
	if err != nil {
		return err
	}
	err = r.consumeReplies()
	if err != nil {
		return err
	}
	klog.V(2).Infof("waiting for requester to ext")
	klog.V(3).Infof("requester will timeout after %s", r.timeout)
	select {
	case done := <-r.done:
		if done == nil {
			log.Println("Tests succeeded, see logs above ^")
			if err := r.shutDown(); err != nil {
				return fmt.Errorf("error during shutdown: %w", err)
			}
		} else {
			if err := r.shutDown(); err != nil {
				return fmt.Errorf("error during shutdown: %w", err)
			}
			return fmt.Errorf("failed due to err %w", done)
		}
	case <-time.After(r.timeout):
		if err := r.shutDown(); err != nil {
			return fmt.Errorf("error during shutdown: %w", err)
		}
		return fmt.Errorf("timed out")
	}
	return nil
}

func (r *Requester) shutDown() error {
	err := r.sendQueue.Shutdown()
	if err != nil {
		return fmt.Errorf("failed to shutdown send q %w", err)
	}
	err = r.receiveQueue.Shutdown(true)
	if err != nil {
		return fmt.Errorf("failed to shutdown rcv q %w", err)
	}
	return nil
}
