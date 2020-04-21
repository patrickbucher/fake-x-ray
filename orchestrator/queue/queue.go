package queue

import (
	"log"
	"time"

	"github.com/streadway/amqp"
)

func MustConnect(amqpURI string, maxRetries int, retryAfter time.Duration) *amqp.Connection {
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
