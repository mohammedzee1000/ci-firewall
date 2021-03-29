package queue

import (
	"encoding/json"
	"fmt"
	"k8s.io/klog/v2"
	"log"

	"github.com/streadway/amqp"
)

type AMQPQueue struct {
	url       string
	conn      *amqp.Connection
	achan     *amqp.Channel
	queueName string
}

func NewAMQPQueue(amqpURI, queueName string) *AMQPQueue {
	return &AMQPQueue{
		url:       amqpURI,
		conn:      nil,
		queueName: queueName,
	}
}

func (aq *AMQPQueue) Init() error {
	var err error
	klog.V(2).Infof("Initializing normal queue %s on provided amqp server", aq.queueName)
	klog.V(3).Infof("Connecting to amqp server %s", aq.url)
	aq.conn, err = amqp.Dial(aq.url)
	if err != nil {
		return fmt.Errorf("failed to dail aqmp server %w", err)
	}
	aq.achan, err = aq.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to get channel from amqp q %w", err)
	}
	_, err = aq.achan.QueueDeclare(aq.queueName, false, true, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to declare queue %w", err)
	}
	return nil
}

func (aq *AMQPQueue) Publish(remotebuild bool, data interface{}) error {
	var err error
	datas, err := json.Marshal(data)
	if err != nil {
		klog.V(4).Infof("failed to marshal data %#v", data)
		return fmt.Errorf("failed to marshal struct %w", err)
	}
	klog.V(2).Infof("publishing data to normal rcv queue")
	publishing := amqp.Publishing{
		Headers:         amqp.Table{},
		ContentType:     "application/json",
		ContentEncoding: "",
		Body:            datas,
		DeliveryMode:    amqp.Transient,
		Priority:        0,
	}
	if remotebuild {
		publishing.AppId = "remote-build"
	}

	return aq.achan.Publish(
		"",
		aq.queueName,
		false,
		false,
		publishing,
	)
}

func (aq *AMQPQueue) Consume(handleDeliveries func(deliveries <-chan amqp.Delivery, done chan error), done chan error) error {
	deliveries, err := aq.achan.Consume(
		aq.queueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("unable to consume %w", err)
	}
	go handleDeliveries(deliveries, done)
	return nil
}

func (aq *AMQPQueue) Shutdown(purge bool) error {
	if purge {
		if _, err := aq.achan.QueuePurge(aq.queueName, false); err != nil {
			log.Printf("failed to purge queue %s nvm failing gracefully: %s\n", aq.queueName, err)
		}
	}
	if err := aq.achan.Close(); err != nil {
		return fmt.Errorf("failed to close channel %w", err)
	}
	if err := aq.conn.Close(); err != nil {
		return fmt.Errorf("failed to close connection amqp %w", err)
	}
	return nil
}
