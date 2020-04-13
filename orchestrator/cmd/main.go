package main

import (
	"net/http"

	"github.com/streadway/amqp"
)

func main() {
	http.HandleFunc("/canary", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fake-x-ray orchestrator up and running"))
	})
	amqp.Dial("localhost:5672/")
	http.ListenAndServe("0.0.0.0:8080", nil)
}
