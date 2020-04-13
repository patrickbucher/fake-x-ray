package main

import (
	"bytes"
	"io"
	"log"
	"net/http"

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

	http.HandleFunc("/score", func(w http.ResponseWriter, r *http.Request) {
		log.Println("/score was called")
		payload := new(bytes.Buffer)
		_, err := io.Copy(payload, r.Body)
		failOn(err)

		// TODO: think carefully about parameters
		err = ch.Publish("", q.Name, false, false, amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "text/plain",
			Body:         payload.Bytes(),
		})
		failOn(err)
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
