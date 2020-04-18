package main

import (
	"bytes"
	"encoding/json"
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

	amqpChannel, err := conn.Channel()
	failOn(err)
	defer amqpChannel.Close()

	// make sure these queues do exist, but do not use its channel
	_, err = amqpChannel.QueueDeclare("body_part", false, false, false, false, nil)
	failOn(err)
	_, err = amqpChannel.QueueDeclare("joints", false, false, false, false, nil)
	failOn(err)
	_, err = amqpChannel.QueueDeclare("scores", false, false, false, false, nil)
	failOn(err)

	xrayQueue, err := amqpChannel.QueueDeclare("xrays", false, false, false, false, nil)
	failOn(err)

	bodyPartDeliveryChannel, err := amqpChannel.Consume("body_part", "", true, false, false, false, nil)
	failOn(err)

	jointDetectionQueue, err := amqpChannel.QueueDeclare("joint_detection", false, false, false, false, nil)
	failOn(err)

	scoreDeliveryChannel, err := amqpChannel.Consume("scores", "", true, false, false, false, nil)
	failOn(err)

	semaphore := make(chan struct{}, 1)
	requestChannels := make(map[string]chan string, 0)

	// TODO: consider using a single func selecting from all the given channels
	go gatherBodyParts(bodyPartDeliveryChannel, requestChannels)
	go gatherScores(scoreDeliveryChannel, requestChannels)

	http.HandleFunc("/score", func(w http.ResponseWriter, r *http.Request) {
		log.Println("/score was called")
		payload := new(bytes.Buffer)
		_, err := io.Copy(payload, r.Body)
		failOn(err)

		corrId, err := uuid.NewRandom()
		failOn(err)
		err = amqpChannel.Publish("", "xrays", false, false, amqp.Publishing{
			ContentType:   "text/plain",
			CorrelationId: corrId.String(),
			ReplyTo:       xrayQueue.Name,
			Body:          payload.Bytes(),
		})
		failOn(err)

		ch := make(chan string, 0)
		semaphore <- struct{}{}
		requestChannels[corrId.String()] = ch
		<-semaphore

		defer func() {
			semaphore <- struct{}{}
			delete(requestChannels, corrId.String())
			<-semaphore
		}()

		result := <-ch

		if result != "HAND_LEFT" {
			w.Write([]byte("payload does not denote a left hand\n"))
			return
		}

		jointDetectionRequest := JointDetectionRequest{
			RawData: payload.String(),
			JointNames: []string{
				"mcp1", "mcp2", "mcp3", "mcp4", "mcp5",
				"pip1", "pip2", "pip3", "pip4", "pip5",
			},
		}
		jointDetectionPayload, err := json.Marshal(jointDetectionRequest)
		failOn(err)

		err = amqpChannel.Publish("", "joint_detection", false, false, amqp.Publishing{
			ContentType:   "text/plain",
			CorrelationId: corrId.String(),
			ReplyTo:       jointDetectionQueue.Name,
			Body:          jointDetectionPayload,
		})
		failOn(err)

		scores := make(map[string]int, 0)
		for i := 0; i < len(jointDetectionRequest.JointNames); i++ {
			scoredJoint := <-ch
			var jointScoreResponse JointScoreResponse
			err = json.Unmarshal([]byte(scoredJoint), &jointScoreResponse)
			failOn(err)

			scores[jointScoreResponse.JointName] = jointScoreResponse.RatingenScore
		}
		clientResponsePayload, err := json.Marshal(scores)
		failOn(err)

		w.Write(clientResponsePayload)
	})

	http.HandleFunc("/canary", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fake-x-ray orchestrator up and running"))
	})

	http.ListenAndServe(httpURI, nil)
}

type JointDetectionRequest struct {
	RawData    string   `json:"raw"`
	JointNames []string `json:"joint_names"`
}

type JointScoreResponse struct {
	JointName     string `json:"joint"`
	RatingenScore int    `json:"score"`
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

func gatherBodyParts(deliveries <-chan amqp.Delivery, requestChannels map[string]chan string) {
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
}

func gatherScores(deliveries <-chan amqp.Delivery, requestChannels map[string]chan string) {
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
}
