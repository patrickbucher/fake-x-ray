package main

import (
	"bytes"
	"io"
	"log"
	"net/http"

	"github.com/streadway/amqp"
)

func main() {
	conn, err := amqp.Dial("amqp://localhost:5672/")
	failOn(err)
	defer conn.Close()

	ch, err := conn.Channel()
	failOn(err)
	defer ch.Close()

	// TODO: think carefully about parameters
	q, err := ch.QueueDeclare("xrays", true, true, true, true, nil)
	failOn(err)

	http.HandleFunc("/score", func(w http.ResponseWriter, r *http.Request) {
		payload := new(bytes.Buffer)
		_, err := io.Copy(payload, r.Body)
		failOn(err)

		// TODO: think carefully about parameters
		ch.Publish("", q.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        payload.Bytes(),
		})
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
