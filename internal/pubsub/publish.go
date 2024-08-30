package pubsub

import (
	"context"
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
			ContentType: "applocation/json",
			Body:        marshaledJSON,
		},
	)
	if err != nil {
		return fmt.Errorf("error publishing message: %v", err)
	}

	return nil
}
