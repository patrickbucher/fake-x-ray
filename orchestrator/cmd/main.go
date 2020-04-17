package main

import (
	"bytes"
	"io"
	"log"
	"net/http"

	uuid "github.com/google/uuid"
	"github.com/streadway/amqp"
)

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOn(err)
	defer conn.Close()

	ch, err := conn.Channel()
	failOn(err)
	defer ch.Close()

	// TODO: think carefully about parameters
	q, err := ch.QueueDeclare("xrays", false, false, false, false, nil)
	failOn(err)

	// TODO: think carefully about parameters
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

	http.ListenAndServe("0.0.0.0:8080", nil)
}

func failOn(err error) {
	if err != nil {
		log.Fatalf("%v", err)
	}
}
