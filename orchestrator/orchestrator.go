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
	"github.com/patrickbucher/fake-x-ray/orchestrator/communication"
	"github.com/patrickbucher/fake-x-ray/orchestrator/queue"
	"github.com/streadway/amqp"
)

const maxRetries = 10
const retryAfter = 2 * time.Second

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

func failOn(err error) {
	if err != nil {
		log.Fatalf("%v", err)
	}
}

func main() {
	amqpURI, httpURI := mustProvideEnvVars()
	conn := queue.MustConnect(amqpURI, maxRetries, retryAfter)
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

	registrations := make(chan communication.ChannelRegistration)
	deregistrations := make(chan string)

	go communication.HandleDeliveries(registrations, deregistrations, bodyPartDeliveryChannel, scoreDeliveryChannel)

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
		registrations <- communication.ChannelRegistration{
			CorrelationID:  corrId.String(),
			RequestChannel: ch,
		}
		defer func() {
			deregistrations <- corrId.String()
		}()

		result := <-ch

		joints, ok := communication.BodyPartJoints[result]

		if !ok {
			w.Write([]byte("payload does not denote a known body part\n"))
			return
		}

		for _, joint := range joints {
			jointDetectionRequest := communication.JointDetectionRequest{
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
			var jointScoreResponse communication.JointScoreResponse
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
