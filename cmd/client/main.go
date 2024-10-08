package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

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

	channel, err := connection.Channel()
	if err != nil {
		log.Fatalf("could not open channel: %v", err)
	}

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatalf("could not get username: %v", err)
	}

	game_state := gamelogic.NewGameState(username)

	err = pubsub.SubscribeJSON(
		connection,
		routing.ExchangePerilDirect,
		routing.PauseKey+"."+game_state.GetUsername(),
		routing.PauseKey,
		pubsub.SimpleQueueTransient,
		handlerPause(game_state),
	)
	if err != nil {
		log.Fatalf("could not subscribe to pause: %v", err)
	}

	err = pubsub.SubscribeJSON(
		connection,
		routing.ExchangePerilTopic,
		routing.ArmyMovesPrefix+"."+game_state.GetUsername(),
		routing.ArmyMovesPrefix+".*",
		pubsub.SimpleQueueTransient,
		handlerArmyMoves(game_state, channel),
	)
	if err != nil {
		log.Fatalf("could not subscribe to pause: %v", err)
	}

	err = pubsub.SubscribeJSON(
		connection,
		routing.ExchangePerilTopic,
		routing.WarRecognitionsPrefix,
		routing.WarRecognitionsPrefix+".*",
		pubsub.SimpleQueueDurable,
		handleWar(game_state, channel),
	)
	if err != nil {
		log.Fatalf("could not subscribe to war: %v", err)
	}

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
			army_move, err := game_state.CommandMove(words)
			if err != nil {
				fmt.Println(err)
				continue
			}

			err = pubsub.PublishJSON(
				channel,
				routing.ExchangePerilTopic,
				routing.ArmyMovesPrefix+"."+game_state.GetUsername(),
				army_move,
			)
			if err != nil {
				log.Printf("could not publish message: %v", err)
				continue
			}

		case "status":
			game_state.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			if len(words) < 2 {
				fmt.Println("usage: spam <number of messages>")
				continue
			}

			number_of_messages, err := strconv.Atoi(words[1])
			if err != nil {
				fmt.Println("number of messages has to be an integer eg: spam 10")
				continue
			}

			for i := 0; i < number_of_messages; i++ {
				err := pubsub.PublishGob(
					channel,
					routing.ExchangePerilTopic,
					routing.GameLogSlug+"."+game_state.GetUsername(),
					routing.GameLog{
						CurrentTime: time.Now(),
						Message:     gamelogic.GetMaliciousLog(),
						Username:    game_state.GetUsername(),
					},
				)
				if err != nil {
					fmt.Printf("could not publish spam message: %v\nspamming stopped\n", err)
					continue
				}
			}
		case "quit":
			gamelogic.PrintQuit()
			return
		default:
			fmt.Println("Me not speak you tongue!? - try using the 'help' command")
		}
	}
}

func handlerPause(game_state *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(playing_state routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		game_state.HandlePause(playing_state)
		return pubsub.Ack
	}
}

func handlerArmyMoves(game_state *gamelogic.GameState, channel *amqp.Channel) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(army_move gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")
		outcome := game_state.HandleMove(army_move)

		switch outcome {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			err := pubsub.PublishJSON(
				channel,
				routing.ExchangePerilTopic,
				routing.WarRecognitionsPrefix+"."+game_state.GetUsername(),
				gamelogic.RecognitionOfWar{
					Attacker: army_move.Player,
					Defender: game_state.GetPlayerSnap(),
				},
			)
			if err != nil {
				log.Printf("could not publish message: %v", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		default:
			fmt.Println("error: unknown move outcome")
			return pubsub.NackDiscard
		}
	}
}

func handleWar(game_state *gamelogic.GameState, channel *amqp.Channel) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(recognition_of_war gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")
		outcome, winner, loser := game_state.HandleWar(recognition_of_war)

		var msg string
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon, gamelogic.WarOutcomeYouWon:
			msg = fmt.Sprintf("%s won a war against %s", winner, loser)
		case gamelogic.WarOutcomeDraw:
			msg = fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser)
		default:
			fmt.Println("error: unknown war outcome")
			return pubsub.NackDiscard
		}

		err := pubsub.PublishGob(
			channel,
			routing.ExchangePerilTopic,
			routing.GameLogSlug+"."+recognition_of_war.Attacker.Username,
			routing.GameLog{
				CurrentTime: time.Now(),
				Message:     msg,
				Username:    game_state.GetUsername(),
			},
		)
		if err != nil {
			return pubsub.NackRequeue
		}

		return pubsub.Ack
	}
}
