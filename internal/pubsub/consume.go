package pubsub

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType int

const (
	SimpleQueueDurable SimpleQueueType = iota
	SimpleQueueTransient
)

type AckType int

const (
	Ack AckType = iota
	NackRequeue
	NackDiscard
)

func DeclareAndBind(
	connection *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType SimpleQueueType, // an enum to represent "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {
	channel, err := connection.Channel()
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("could not open channel: %v", err)
	}

	queue, err := channel.QueueDeclare(
		queueName,
		simpleQueueType == SimpleQueueDurable,
		simpleQueueType != SimpleQueueDurable,
		simpleQueueType != SimpleQueueDurable,
		false,
		amqp.Table{
			"x-dead-letter-exchange": "peril_dlx",
		},
	)
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("could not declare queue: %v", err)
	}

	channel.QueueBind(
		queue.Name,
		key,
		exchange,
		false,
		nil,
	)

	return channel, queue, nil
}

func SubscribeJSON[T any](
	connection *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType SimpleQueueType,
	handler func(T) AckType,
) error {
	JSONunmarshaller := func(data []byte) (T, error) {
		var msg T
		err := json.Unmarshal(data, &msg)
		if err != nil {
			return msg, fmt.Errorf("could not unmarshal JSON message: %v", err)
		}
		return msg, nil
	}

	return subscribe(
		connection,
		exchange,
		queueName,
		key,
		simpleQueueType,
		handler,
		JSONunmarshaller,
	)
}

func SubscribeGob[T any](
	connection *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType SimpleQueueType,
	handler func(T) AckType,
) error {
	GOBdecoder := func(data []byte) (T, error) {
		var msg T
		decoder := gob.NewDecoder(bytes.NewReader(data))
		err := decoder.Decode(&msg)
		if err != nil {
			return msg, fmt.Errorf("could not decode GOB message: %v", err)
		}
		return msg, nil
	}

	return subscribe(
		connection,
		exchange,
		queueName,
		key,
		simpleQueueType,
		handler,
		GOBdecoder,
	)
}

func subscribe[T any](
	connection *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType SimpleQueueType,
	handler func(T) AckType,
	unmarshaller func([]byte) (T, error),
) error {
	channel, _, err := DeclareAndBind(
		connection,
		exchange,
		queueName,
		key,
		simpleQueueType,
	)
	if err != nil {
		return fmt.Errorf("could not declare and bind queue: %v", err)
	}

	err = channel.Qos(10, 0, false)
	if err != nil {
		return fmt.Errorf("could not set QoS: %v", err)
	}

	consume_channel, err := channel.Consume(
		queueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("could not consume queue: %v", err)
	}

	go func() {
		defer channel.Close()
		for delivery := range consume_channel {
			msg, err := unmarshaller(delivery.Body)
			if err != nil {
				log.Print(err)
				continue
			}
			ack := handler(msg)
			switch ack {
			case Ack:
				delivery.Ack(false)

			case NackRequeue:
				delivery.Nack(false, true)

			case NackDiscard:
				delivery.Nack(false, false)
			}
		}
	}()

	return nil
}
