/*
 * Copyright 2019 The NATS Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"strings"
	"time"

	"github.com/nats-io/nats-kafka/server/conf"
	"github.com/nats-io/nats-kafka/server/core"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nuid"
	"github.com/segmentio/kafka-go"
)

var iterations int
var chunk int
var kafkaHostPort string
var natsURL string

func startBridge(connections []conf.ConnectorConfig) (*core.NATSKafkaBridge, error) {
	config := conf.DefaultBridgeConfig()
	config.Logging.Debug = true
	config.Logging.Trace = true
	config.Logging.Colors = false
	config.Monitoring = conf.HTTPConfig{
		HTTPPort: -1,
	}
	config.NATS = conf.NATSConfig{
		Servers:        []string{natsURL},
		ConnectTimeout: 2000,
		ReconnectWait:  2000,
		MaxReconnects:  5,
	}

	for i, c := range connections {
		c.Brokers = []string{kafkaHostPort}
		connections[i] = c
	}

	config.Connect = connections

	bridge := core.NewNATSKafkaBridge()
	err := bridge.InitializeFromConfig(config)
	if err != nil {
		return nil, err
	}
	err = bridge.Start()
	if err != nil {
		bridge.Stop()
		return nil, err
	}

	return bridge, nil
}

func main() {
	flag.IntVar(&iterations, "i", 100, "iterations, defaults to 100")
	flag.IntVar(&chunk, "c", 1, "messages per write, chunk size, defaults to 1")
	flag.StringVar(&kafkaHostPort, "kafka", "localhost:9092", "kafka host:port, defaults to localhost:9092")
	flag.StringVar(&natsURL, "nats", "nats://localhost:4222", "nats url, defaults to nats://localhost:4222")
	flag.Parse()

	subject := nuid.Next()
	topic := nuid.Next()
	msgString := strings.Repeat("stannats", 128) // 1024 bytes
	msg := []byte(msgString)
	msgLen := len(msg)

	connect := []conf.ConnectorConfig{
		{
			Type:    "KafkaToNATS",
			Subject: subject,
			Topic:   topic,
		},
	}

	log.Printf("creating topic %s", topic)
	connection, err := kafka.DialContext(context.Background(), "tcp", kafkaHostPort)
	if connection == nil || err != nil {
		log.Fatalf("unable to connect to kafka server")
	}

	connection.SetDeadline(time.Now().Add(15 * time.Second))
	err = connection.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
	connection.Close()
	if err != nil {
		log.Fatalf("error creating topic, %s", err.Error())
	}

	bridge, err := startBridge(connect)
	if err != nil {
		log.Fatalf("error starting bridge, %s", err.Error())
	}

	done := make(chan bool)
	count := 0
	interval := int(iterations / 10)

	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("error connecting to nats, %s", err.Error())
	}
	defer nc.Close()

	nc.Subscribe(subject, func(msg *nats.Msg) {
		count++
		if count%interval == 0 {
			log.Printf("received count = %d", count)
		}

		if len(msg.Data) != msgLen {
			log.Fatalf("received message that is the wrong size %d != %d", len(msg.Data), msgLen)
		}

		if count == iterations {
			done <- true
		}
	})

	log.Printf("sending %d messages through Kafka to the bridge to NATS, in chunks of %d...", iterations, chunk)

	start := time.Now()
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  []string{kafkaHostPort},
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	})

	for i := 0; i < iterations/chunk; i++ {
		msgs := []kafka.Message{}
		for j := 0; j < chunk; j++ {
			msgs = append(msgs, kafka.Message{
				Key:   []byte(topic),
				Value: msg,
			})
		}
		err := writer.WriteMessages(context.Background(), msgs...)
		if err != nil {
			log.Fatalf("error putting messages on topic, %s", err.Error())
		}
		if (i*chunk)%interval == 0 {
			log.Printf("%s: send count = %d", topic, (i+1)*chunk)
		}
	}
	writer.Close()
	<-done
	end := time.Now()

	stats := bridge.SafeStats()
	statsJSON, _ := json.MarshalIndent(stats, "", "    ")

	bridge.Stop()

	diff := end.Sub(start)
	rate := float64(iterations) / float64(diff.Seconds())
	log.Printf("Bridge Stats:\n\n%s\n", statsJSON)
	log.Printf("Sent %d messages through a kafka topic to a NATS subscriber in %s, or %.2f msgs/sec", iterations, diff, rate)
}
