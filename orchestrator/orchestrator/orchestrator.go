package orchestrator

import "github.com/streadway/amqp"

var BodyPartJoints = map[string][]string{
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

func HandleDeliveries(registrations chan ChannelRegistration, deregistrations chan string,
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
