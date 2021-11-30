// Copyright 2016 Michael Stapelberg and contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package mayqtt implements an MQTT client which receives scan requests from
// scan2drive/cmd/scan and publishes status to scan2drive/ui/status.
package mayqtt

import (
	"encoding/json"
	"fmt"
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stapelberg/scan2drive"
	"golang.org/x/net/trace"
)

type scanRequest struct {
	User   string `json:"user"`
	Source string `json:"source"`
}

type PublishRequest struct {
	Topic    string
	Qos      byte
	Retained bool
	Payload  interface{}
}

func mqttLoop(mqttScanRequests chan *scan2drive.ScanRequest, requests <-chan PublishRequest) error {
	tr := trace.New("MQTT", "Loop")
	defer tr.Finish()

	const broker = "tcp://dr.lan:1883"
	tr.LazyPrintf("Connecting to MQTT broker %s", broker)
	opts := mqtt.NewClientOptions().AddBroker(broker)
	opts.SetClientID("scan2drive")
	opts.SetConnectRetry(true)
	opts.OnConnect = func(c mqtt.Client) {
		tr.LazyPrintf("OnConnect, subscribing to scan2drive/cmd/scan")
		token := c.Subscribe(
			"scan2drive/cmd/scan",
			0, /* qos */
			func(_ mqtt.Client, m mqtt.Message) {
				tr.LazyPrintf("message on topic %s: %q", m.Topic(), string(m.Payload()))
				var sr scan2drive.ScanRequest
				if err := json.Unmarshal(m.Payload(), &sr); err != nil {
					log.Printf("error unmarshaling payload: %v", err)
				}
				select {
				case mqttScanRequests <- &sr:
				default:
					// Channel full, scan request already pending; drop
				}
			})
		if token.Wait() && token.Error() != nil {
			tr.LazyPrintf("subscription failed! %v", token.Error())
		}
	}
	mqttClient := mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("MQTT connection failed: %v", token.Error())
	}
	tr.LazyPrintf("Connected to MQTT broker %s", broker)

	for r := range requests {
		tr.LazyPrintf("publishing on topic %s: %q", r.Topic, r.Payload)
		// discard Token, MQTT publishing is best-effort
		_ = mqttClient.Publish(r.Topic, r.Qos, r.Retained, r.Payload)
	}
	return nil
}

var publish chan PublishRequest

func MQTT(scanRequests chan *scan2drive.ScanRequest) {
	publish = make(chan PublishRequest)
	go func() {
		if err := mqttLoop(scanRequests, publish); err != nil {
			log.Print(err)
		}
	}()
}

var lastStatus string

func Publishf(format string, args ...interface{}) {
	status := fmt.Sprintf(format, args...)
	// Prevent duplicate messages if status has not changed
	if lastStatus == status {
		return
	}
	lastStatus = status
	select {
	case publish <- PublishRequest{
		Topic:    "scan2drive/ui/status",
		Retained: true,
		Payload:  []byte(status),
	}:
	default:
		// drop message if MQTT is not connected
	}
}
