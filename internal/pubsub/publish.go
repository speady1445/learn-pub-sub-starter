package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	marshaledJSON, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("error marshalling JSON: %v", err)
	}

	err = ch.PublishWithContext(
		context.Background(),
		exchange,
		key,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        marshaledJSON,
		},
	)
	if err != nil {
		return fmt.Errorf("error publishing message: %v", err)
	}

	return nil
}

func PublishGob[T any](channel *amqp.Channel, exchange, key string, value T) error {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	err := encoder.Encode(value)
	if err != nil {
		return fmt.Errorf("error encoding gob: %v", err)
	}

	err = channel.PublishWithContext(
		context.Background(),
		exchange,
		key,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/gob",
			Body:        buffer.Bytes(),
		},
	)
	if err != nil {
		return fmt.Errorf("error publishing message: %v", err)
	}

	return nil
}
