package pubsub

import (
	"context"
	"encoding/json"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType string
type SimpleAckType string

const (
	Durable   SimpleQueueType = "durable"
	Transient SimpleQueueType = "transient"
)

const (
	Ack         SimpleAckType = "ack"
	NackRequeue SimpleAckType = "nack_requeue"
	NackDiscard SimpleAckType = "nack_discard"
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	mVal, err := json.Marshal(val)
	if err != nil {
		return err
	}

	ctx := context.Background()
	publishing := amqp.Publishing{
		ContentType: "application/json",
		Body:        mVal,
	}

	ch.PublishWithContext(ctx, exchange, key, false, false, publishing)

	return nil
}

func DeclareAndBind(conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType) (*amqp.Channel, amqp.Queue, error) {
	ch, err := conn.Channel()
	if err != nil {
		log.Fatal("Could not connect create a channel")
		return nil, amqp.Queue{}, err
	}

	durable, autoDelete, exclusive := false, false, false

	switch queueType {
	case Durable:
		durable = true
	case Transient:
		autoDelete = true
		exclusive = true
	}

	table := amqp.Table{
		"x-dead-letter-exchange":    "peril_dlx",
		"x-dead-letter-routing-key": "",
	}

	queue, err := ch.QueueDeclare(queueName, durable, autoDelete, exclusive, false, table)
	if err != nil {
		log.Fatal("Could not connect declare queue")
		return nil, amqp.Queue{}, err
	}

	err = ch.QueueBind(queueName, key, exchange, false, nil)

	return ch, queue, nil
}

func SubscribeJSON[T any](conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType, handler func(T) SimpleAckType) error {
	ch, _, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return err
	}

	deliveryChan, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		for deliveries := range deliveryChan {
			var val T
			err := json.Unmarshal(deliveries.Body, &val)
			if err != nil {
				log.Panicf("could not unmarshal delivery channel: %v", err)
			}

			ackType := handler(val)

			switch ackType {
			case Ack:
				log.Print("messaged acked")
				deliveries.Ack(false)
			case NackRequeue:
				log.Print("messaged nacked, requeue")
				deliveries.Nack(false, true)
			case NackDiscard:
				log.Print("messaged nacked, discard")
				deliveries.Nack(false, false)
			}
		}
	}()

	return nil
}
