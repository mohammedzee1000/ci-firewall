package sendq

import (
	"fmt"
	"github.com/mohammedzee1000/ci-firewall/pkg/ci-firewall/cli/genericclioptions"
	"github.com/mohammedzee1000/ci-firewall/pkg/queue"
	"github.com/spf13/cobra"
	"os"
)

const SendQueueCreateRecommendedCommandName = "create"

type SenqQCreateOptions struct {
	amqpURI          string
	sendQName        string
	sendExchangeName string
	sendTopic        string
}

func NewSendQueueCreateOptions() *SenqQCreateOptions {
	return &SenqQCreateOptions{}
}

func (sqco *SenqQCreateOptions) Complete(name string, cmd *cobra.Command, args []string) error {
	return  nil
}

func (sqco *SenqQCreateOptions) Validate() (err error) {
	if sqco.amqpURI == "" {
		return fmt.Errorf("provide AMQP URI")
	}
	if sqco.sendExchangeName == "" {
		return fmt.Errorf("please provide send exchange name")
	}
	if sqco.sendTopic == "" {
		return fmt.Errorf("please provide send q topic")
	}
	return nil
}

func (sqco *SenqQCreateOptions) Run() (err error) {
	fmt.Printf("[Idempotent] Creating send queue with name %s, with which will be bound to exchange %s, using routing key/topic %s", sqco.sendQName, sqco.sendExchangeName, sqco.sendTopic)
	jq := queue.NewJMSAMQPQueue(sqco.amqpURI, sqco.sendQName, sqco.sendExchangeName, sqco.sendTopic)
	err = jq.Init()
	if err != nil {
		return fmt.Errorf("failed to initialize queue %w", err)
	}
	err = jq.Shutdown()
	if err != nil {
		return fmt.Errorf("failed to shutdown connection %w", err)
	}
	fmt.Println("queue initialized/verified, please check with broker to verify")
	return nil
}

func NewCmdSendQueueCreate(name, fullname string) *cobra.Command  {
	o := NewSendQueueCreateOptions()
	cmd := &cobra.Command{
		Use: name,
		Short: "initialize a send queue",
		Run: func(cmd *cobra.Command, args []string) {
			genericclioptions.GenericRun(o, cmd, args)
		},
	}
	cmd.Flags().StringVar(&o.amqpURI, "amqpuri", os.Getenv("AMQP_URI"), "the url of amqp server")
	cmd.Flags().StringVar(&o.sendQName, "sendqueue", "amqp.ci.queue.send", "the name of the send queue")
	cmd.Flags().StringVar(&o.sendExchangeName, "sendexchange", "amqp.ci.exchange.send", "the name of the exchange tp use for send")
	cmd.Flags().StringVar(&o.sendTopic, "sendtopic", "amqp.ci.topic.send", "the name of the send topic")
	return cmd
}