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
	fmt.Println("Starting Peril client...")
	connection_string := "amqp://guest:guest@localhost:5672/"

	connection, err := amqp.Dial(connection_string)
	if err != nil {
		log.Fatalf("Could not connect to RabbitMQ: %v", err)
	}
	defer connection.Close()
	fmt.Println("Successfully connected to RabbitMQ!")

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatalf("could not get username: %v", err)
	}

	_, _, err = pubsub.DeclareAndBind(
		connection,
		routing.ExchangePerilDirect,
		fmt.Sprintf("%s.%s", routing.PauseKey, username),
		routing.PauseKey,
		pubsub.SimpleQueueTransient,
	)
	if err != nil {
		log.Fatalf("could not subscribe to pause: %v", err)
	}

	game_state := gamelogic.NewGameState(username)

	gamelogic.PrintClientHelp()

	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}

		switch words[0] {
		case "spawn":
			err := game_state.CommandSpawn(words)
			if err != nil {
				fmt.Println(err)
			}
		case "move":
			_, err := game_state.CommandMove(words)
			if err != nil {
				fmt.Println(err)
			}
		case "status":
			game_state.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			fmt.Println("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			return
		default:
			fmt.Println("Me not speak you tongue!? - try using the 'help' command")
		}
	}
}
