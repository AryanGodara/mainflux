// Copyright (c) Mainflux
// SPDX-License-Identifier: Apache-2.0

package rabbitmq_test

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"testing"

	"github.com/gogo/protobuf/proto"
	logg "github.com/mainflux/mainflux/logger"
	"github.com/mainflux/mainflux/pkg/messaging"
	"github.com/mainflux/mainflux/pkg/messaging/rabbitmq"
	dockertest "github.com/ory/dockertest/v3"
	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	publisher messaging.Publisher
	pubsub    messaging.PubSub
	logger    logg.Logger
)

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	container, err := pool.Run("rabbitmq", "3.9.20", []string{})
	if err != nil {
		log.Fatalf("Could not start container: %s", err)
	}
	handleInterrupt(pool, container)

	address := fmt.Sprintf("amqp://%s:%s", "localhost", container.GetPort("5672/tcp"))
	if err := pool.Retry(func() error {
		publisher, err = rabbitmq.NewPublisher(address)
		return err
	}); err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	logger, err = logg.New(os.Stdout, "error")
	if err != nil {
		log.Fatalf(err.Error())
	}
	if err := pool.Retry(func() error {
		pubsub, err = rabbitmq.NewPubSub(address, "mainflux", logger)
		return err
	}); err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	code := m.Run()
	if err := pool.Purge(container); err != nil {
		log.Fatalf("Could not purge container: %s", err)
	}

	os.Exit(code)
}

func rabbitHandler(deliveries <-chan amqp.Delivery, h messaging.MessageHandler) {
	for d := range deliveries {
		var msg messaging.Message
		if err := proto.Unmarshal(d.Body, &msg); err != nil {
			logger.Warn(fmt.Sprintf("Failed to unmarshal received message: %s", err))
			return
		}
		if err := h.Handle(msg); err != nil {
			logger.Warn(fmt.Sprintf("Failed to handle Mainflux message: %s", err))
			return
		}
	}
}

func handleInterrupt(pool *dockertest.Pool, container *dockertest.Resource) {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		if err := pool.Purge(container); err != nil {
			log.Fatalf("Could not purge container: %s", err)
		}
		os.Exit(0)
	}()
}
