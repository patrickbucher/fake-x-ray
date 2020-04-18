package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/streadway/amqp"
)

const maxRetries = 10
const retryAfter = 2 * time.Second

func main() {
	amqpURI, httpURI := mustProvideEnvVars()
	conn := mustConnect(amqpURI, maxRetries, retryAfter)
	defer conn.Close()

	ch, err := conn.Channel()
	failOn(err)
	defer ch.Close()

	// make sure queue does exist, but do not use its channel
	_, err = ch.QueueDeclare("body_part", false, false, false, false, nil)
	failOn(err)

	q, err := ch.QueueDeclare("xrays", false, false, false, false, nil)
	failOn(err)

	deliveryChannel, err := ch.Consume("body_part", "", true, false, false, false, nil)
	failOn(err)

	semaphore := make(chan struct{}, 1)
	requestChannels := make(map[string]chan string, 0)

	go func(deliveries <-chan amqp.Delivery) {
		for {
			select {
			case delivery := <-deliveries:
				requestChannel, ok := requestChannels[delivery.CorrelationId]
				if !ok {
					log.Fatalf("unable to find request channel with correlation ID %s", delivery.CorrelationId)
				}
				requestChannel <- string(delivery.Body)
			}
		}
	}(deliveryChannel)

	http.HandleFunc("/score", func(w http.ResponseWriter, r *http.Request) {
		log.Println("/score was called")
		payload := new(bytes.Buffer)
		_, err := io.Copy(payload, r.Body)
		failOn(err)

		corrId, err := uuid.NewRandom()
		failOn(err)
		err = ch.Publish("", "xrays", false, false, amqp.Publishing{
			ContentType:   "text/plain",
			CorrelationId: corrId.String(),
			ReplyTo:       q.Name,
			Body:          payload.Bytes(),
		})
		failOn(err)

		ch := make(chan string, 0)
		semaphore <- struct{}{}
		requestChannels[corrId.String()] = ch
		<-semaphore

		result := <-ch

		semaphore <- struct{}{}
		delete(requestChannels, corrId.String())
		<-semaphore

		w.Write([]byte(result))
	})

	http.HandleFunc("/canary", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fake-x-ray orchestrator up and running"))
	})

	http.ListenAndServe(httpURI, nil)
}

func failOn(err error) {
	if err != nil {
		log.Fatalf("%v", err)
	}
}

func mustProvideEnvVars() (amqpURI, httpURI string) {
	amqpURI = os.Getenv("AMQP_URI")
	if amqpURI == "" {
		log.Fatalf("environment variable AMQP_URI not set")
	}
	httpURI = os.Getenv("HTTP_URI")
	if httpURI == "" {
		log.Fatalf("environment variable HTTP_URI not set")
	}
	return amqpURI, httpURI
}

func mustConnect(amqpURI string, maxRetries int, retryAfter time.Duration) *amqp.Connection {
	connected := false
	retries := 0
	var conn *amqp.Connection
	var err error
	for !connected && retries < maxRetries {
		conn, err = amqp.Dial(amqpURI)
		retries++
		if err != nil {
			log.Printf("unable to connect to RabbitMQ on '%s', waiting...", amqpURI)
			time.Sleep(retryAfter)
			continue
		} else {
			connected = true
		}
	}
	if !connected {
		log.Fatalf("unable to connect to '%s' after %d retries", amqpURI, maxRetries)
	}
	return conn
}
