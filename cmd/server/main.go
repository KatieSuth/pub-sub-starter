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
	log.Println("Starting Peril server...")
	gamelogic.PrintServerHelp()

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

	key := fmt.Sprintf("%s.*", routing.GameLogSlug)

	pubsub.DeclareAndBind(connection, routing.ExchangePerilTopic, routing.GameLogSlug, key, pubsub.Durable)

	for {
		input := gamelogic.GetInput()

		if len(input) == 0 {
			continue
		}

		shouldBreak := false
		switch strings.ToLower(input[0]) {
		case "pause":
			log.Printf("sending pause message")
			err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})
			if err != nil {
				log.Fatal("Could not publish message")
				return
			}
		case "resume":
			log.Printf("sending resume message")
			err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: false})
			if err != nil {
				log.Fatal("Could not publish message")
				return
			}
		case "quit":
			log.Printf("exiting")
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

	log.Println("Shutting down Peril server")
}
