package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	log.Println("Starting Peril client...")

	connectStr := "amqp://guest:guest@localhost:5672/"

	connection, err := amqp.Dial(connectStr)
	if err != nil {
		log.Fatalf("Could not connect to amqp server: %v", err)
		return
	}
	defer connection.Close()
	log.Println("AQMP connection successful")

	ch, err := connection.Channel()
	if err != nil {
		log.Fatal("Could not connect create a channel")
	}

	/*
		err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})
		if err != nil {
			log.Fatal("Could not publish message")
		}
	*/

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatalf("Could not establish user: %v", err)
		return
	}

	gameState := gamelogic.NewGameState(username)
	queueName := fmt.Sprintf("%s.%s", routing.PauseKey, username)

	//pubsub.DeclareAndBind(connection, routing.ExchangePerilDirect, queueName, routing.PauseKey, pubsub.Transient)
	pubsub.SubscribeJSON(connection, routing.ExchangePerilDirect, queueName, routing.PauseKey, pubsub.Transient, handlerPause(gameState))

	moveKey := fmt.Sprintf("%s.*", routing.ArmyMovesPrefix)
	moveQueue := fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, username)
	pubsub.SubscribeJSON(connection, routing.ExchangePerilTopic, moveQueue, moveKey, pubsub.Transient, handlerMove(gameState))

	for {
		input := gamelogic.GetInput()

		if len(input) == 0 {
			continue
		}

		shouldBreak := false
		switch strings.ToLower(input[0]) {
		case "spawn":
			err = gameState.CommandSpawn(input)
			if err != nil {
				log.Printf("Could not spawn units: %v", err)
			}
		case "move":
			move, err := gameState.CommandMove(input)
			if err != nil {
				log.Printf("Could not move army: %v", err)
			}
			err = pubsub.PublishJSON(ch, routing.ExchangePerilTopic, moveKey, move)
			log.Printf("Move successful")
		case "status":
			gameState.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			fmt.Println("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			shouldBreak = true
		default:
			log.Printf("command not recognized")
		}

		if shouldBreak {
			break
		}
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan

	log.Println("Shutting down Peril client")
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.SimpleAckType {
	return func(ps routing.PlayingState) pubsub.SimpleAckType {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState) func(gamelogic.ArmyMove) pubsub.SimpleAckType {
	return func(move gamelogic.ArmyMove) pubsub.SimpleAckType {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(move)

		if outcome == gamelogic.MoveOutComeSafe || outcome == gamelogic.MoveOutcomeMakeWar {
			return pubsub.Ack
		} else {
			return pubsub.NackDiscard
		}
	}
}
