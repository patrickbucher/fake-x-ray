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

var bodyPartJoints = map[string][]string{
	"HAND_LEFT": []string{
		"mcp1",
		"mcp2",
		"mcp3",
		"mcp4",
		"mcp5",
		"pip1",
		"pip2",
		"pip3",
		"pip4",
		"pip5",
	},
}

type JointDetectionRequest struct {
	RawData    string   `json:"raw"`
	JointNames []string `json:"joint_names"`
}

type JointScoreResponse struct {
	JointName     string `json:"joint"`
	RatingenScore int    `json:"score"`
}

type ChannelRegistration struct {
	CorrelationID  string
	RequestChannel chan string
}

func handleDeliveries(registrations chan ChannelRegistration, deregistrations chan string,
	bodyPartDeliveryChannel, scoreDeliveryChannel <-chan amqp.Delivery) {
	requestChannels := make(map[string]chan string, 0)
	for {
		select {
		case reg := <-registrations:
			requestChannels[reg.CorrelationID] = reg.RequestChannel
		case corrID := <-deregistrations:
			if ch, ok := requestChannels[corrID]; ok {
				close(ch)
				delete(requestChannels, corrID)
			}
		case delivery := <-bodyPartDeliveryChannel:
			if requestChannel, ok := requestChannels[delivery.CorrelationId]; ok {
				requestChannel <- string(delivery.Body)
			}
		case delivery := <-scoreDeliveryChannel:
			if requestChannel, ok := requestChannels[delivery.CorrelationId]; ok {
				requestChannel <- string(delivery.Body)
			}

		}
	}
}

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

	registrations := make(chan ChannelRegistration)
	deregistrations := make(chan string)

	go handleDeliveries(registrations, deregistrations, bodyPartDeliveryChannel, scoreDeliveryChannel)

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
		registrations <- ChannelRegistration{
			CorrelationID:  corrId.String(),
			RequestChannel: ch,
		}
		defer func() {
			deregistrations <- corrId.String()
		}()

		result := <-ch

		joints, ok := bodyPartJoints[result]

		if !ok {
			w.Write([]byte("payload does not denote a known body part\n"))
			return
		}

		for _, joint := range joints {
			jointDetectionRequest := JointDetectionRequest{
				RawData:    payload.String(),
				JointNames: []string{joint},
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
		}

		scores := make(map[string]int, 0)
		for i := 0; i < len(joints); i++ {
			scoredJoint := <-ch
			var jointScoreResponse JointScoreResponse
			err = json.Unmarshal([]byte(scoredJoint), &jointScoreResponse)
			failOn(err)

			if jointScoreResponse.RatingenScore == -1 {
				continue
			}
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
