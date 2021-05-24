package main

import (
	"encoding/json"
	"fmt"
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type scanRequest struct {
	User   string `json:"user"`
	Source string `json:"source"`
}

type publishRequest struct {
	Topic    string
	Qos      byte
	Retained bool
	Payload  interface{}
}

func publisherLoop(requests <-chan publishRequest) error {
	const broker = "tcp://dr.lan:1883"
	log.Printf("Connecting to MQTT broker %q", broker)
	opts := mqtt.NewClientOptions().AddBroker(broker)
	opts.SetClientID("scan2drive")
	opts.SetConnectRetry(true)
	mqttClient := mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("MQTT connection failed: %v", token.Error())
	}

	token := mqttClient.Subscribe(
		"scan2drive/cmd/scan",
		0, /* qos */
		func(_ mqtt.Client, m mqtt.Message) {
			log.Printf("message on topic %s: %q", m.Topic(), string(m.Payload()))
			var sr scanRequest
			if err := json.Unmarshal(m.Payload(), &sr); err != nil {
				log.Printf("error unmarshaling payload: %v", err)
			}
			select {
			case mqttScanRequest <- sr:
			default:
				// Channel full, scan request already pending; drop
			}
		})
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	for r := range requests {
		// discard Token, MQTT publishing is best-effort
		_ = mqttClient.Publish(r.Topic, r.Qos, r.Retained, r.Payload)
	}
	return nil
}

func MQTT() chan<- publishRequest {
	result := make(chan publishRequest)
	go func() {
		if err := publisherLoop(result); err != nil {
			log.Print(err)
		}
	}()
	return result
}
