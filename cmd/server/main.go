package main

import (
	"fmt"
	"log"

	"github.com/speady1445/learn-pub-sub-starter/internal/gamelogic"
	"github.com/speady1445/learn-pub-sub-starter/internal/pubsub"
	"github.com/speady1445/learn-pub-sub-starter/internal/routing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril server...")
	connection_string := "amqp://guest:guest@localhost:5672/"
	connection, err := amqp.Dial(connection_string)
	if err != nil {
		log.Fatalf("could not connect to RabbitMQ: %v", err)
	}
	defer connection.Close()
	fmt.Println("Successfully connected to RabbitMQ!")

	channel, err := connection.Channel()
	if err != nil {
		log.Fatalf("could not open channel: %v", err)
	}

	_, _, err = pubsub.DeclareAndBind(
		connection,
		routing.ExchangePerilTopic,
		routing.GameLogSlug,
		routing.GameLogSlug+".*",
		pubsub.SimpleQueueDurable,
	)
	if err != nil {
		log.Fatalf("could not subscribe to game logs: %v", err)
	}

	gamelogic.PrintServerHelp()

	for {
		words := gamelogic.GetInput()

		if len(words) == 0 {
			continue
		}

		switch words[0] {
		case "pause":
			fmt.Println("Pausing...")
			pause(channel)
		case "resume":
			fmt.Println("Resuming...")
			unpause(channel)
		case "quit":
			return
		case "help":
			gamelogic.PrintServerHelp()
		default:
			fmt.Println("Me not speak you tongue!?")
		}
	}
}

func pause(channel *amqp.Channel) {
	err := pubsub.PublishJSON(
		channel,
		routing.ExchangePerilDirect,
		routing.PauseKey,
		routing.PlayingState{IsPaused: true},
	)
	if err != nil {
		log.Printf("could not publish message: %v", err)
	}
}

func unpause(channel *amqp.Channel) {
	err := pubsub.PublishJSON(
		channel,
		routing.ExchangePerilDirect,
		routing.PauseKey,
		routing.PlayingState{IsPaused: false},
	)
	if err != nil {
		log.Printf("could not publish message: %v", err)
	}
}
