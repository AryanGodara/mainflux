// Copyright (c) Mainflux
// SPDX-License-Identifier: Apache-2.0

package mqtt_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/mainflux/mainflux/pkg/messaging"
	mqtt_pubsub "github.com/mainflux/mainflux/pkg/messaging/mqtt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	topic       = "topic"
	chansPrefix = "channels"
	channel     = "9b7b1b3f-b1b0-46a8-a717-b8213f9eda3b"
	subtopic    = "engine"
)

var (
	msgChan = make(chan messaging.Message)
	data    = []byte("payload")
)

// Tests the MQTT Broker being used for the tests
func TestMQTTBroker(t *testing.T) {
	// Subscribing with topic, and with subtopic, so that we can publish messages
	client, err := newClient(address, "clientID", 30*time.Second)
	t.Cleanup(func() {
		defer client.Unsubscribe()
		defer client.Disconnect(5)
	})
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))

	token := client.Subscribe(topic, qos, mqttHandler(handler{false}))
	token.Wait()
	require.Nil(t, token.Error(), fmt.Sprintf("got unexpected error: %s", token.Error()))
	token = client.Subscribe(fmt.Sprintf("%s.%s", topic, subtopic), qos, nil)
	require.Nil(t, token.Error(), fmt.Sprintf("got unexpected error: %s", token.Error()))

	cases := []struct {
		desc    string
		topic   string
		message []byte
	}{
		{
			desc:    "publish nil message to topic",
			topic:   topic,
			message: nil,
		},
		{
			desc:    "publish nil message to topic.subtopic",
			topic:   fmt.Sprintf("%s.%s", topic, subtopic),
			message: nil,
		},
		{
			desc:    "publishing to topic",
			topic:   topic,
			message: data,
		},
		{
			desc:    "publishing to topic.subtopic",
			topic:   fmt.Sprintf("%s.%s", topic, subtopic),
			message: data,
		},
	}
	for _, tc := range cases {
		token = client.Publish(tc.topic, qos, false, tc.message)
		token.Wait()
		receivedMsg := <-msgChan
		if tc.message == nil {
			assert.Equal(t, 0, len(receivedMsg.Payload), fmt.Sprintf("%s: expected nil message, but got: %s", tc.desc, string(receivedMsg.Payload)))
		} else {
			assert.Equal(t, tc.message, receivedMsg.Payload, fmt.Sprintf("expected %s, got %s", tc.message, &receivedMsg.Payload))
		}
	}

	token = client.Unsubscribe(topic)
	token.Wait()
	require.Nil(t, token.Error(), fmt.Sprintf("got unexpected error: %s", token.Error()))
	token = client.Unsubscribe(fmt.Sprintf("%s.%s", topic, subtopic))
	token.Wait()
	require.Nil(t, token.Error(), fmt.Sprintf("got unexpected error: %s", token.Error()))
}

// Tests only the Publisher
func TestPublisher(t *testing.T) {
	// Subscribing with topic, and with subtopic, so that we can publish messages
	client, err := newClient(address, "clientID1", 30*time.Second)
	t.Cleanup(func() {
		defer client.Unsubscribe()
		defer client.Disconnect(5)
	})
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))

	token := client.Subscribe(topic, qos, nil)
	assert.Nil(t, token.Error(), fmt.Sprintf("got unexpected error: %s", token.Error()))
	token = client.Subscribe(fmt.Sprintf("%s.%s", topic, subtopic), qos, nil)
	assert.Nil(t, token.Error(), fmt.Sprintf("got unexpected error: %s", token.Error()))

	// publish with empty topic
	err = publisher.Publish("", messaging.Message{Payload: data})
	assert.Equal(t, err, mqtt_pubsub.ErrEmptyTopic, fmt.Sprintf("Publish with empty topic: expected: %s, got: %s", mqtt_pubsub.ErrEmptyTopic, err))

	cases := []struct {
		desc     string
		topic    string
		channel  string
		subtopic string
		payload  []byte
	}{
		{
			desc:    "publish message with nil payload",
			payload: nil,
		},
		{
			desc:    "publish message with string payload",
			payload: data,
		},
		{
			desc:    "publish message with channel",
			payload: data,
			channel: channel,
		},
		{
			desc:     "publish message with subtopic",
			payload:  data,
			subtopic: subtopic,
		},
		{
			desc:     "publish message with channel and subtopic",
			payload:  data,
			channel:  channel,
			subtopic: subtopic,
		},
	}
	for _, tc := range cases {
		expectedMsg := messaging.Message{
			Channel:  tc.channel,
			Subtopic: tc.subtopic,
			Payload:  tc.payload,
		}
		err := publisher.Publish(topic, expectedMsg)
		assert.Nil(t, err, fmt.Sprintf("%s: got unexpected error: %s\n", tc.desc, err))

		receivedMsg := <-msgChan
		if tc.payload == nil {
			assert.Equal(t, 0, len(receivedMsg.Payload), fmt.Sprintf("%s: expected %+v got %+v\n", tc.desc, tc.payload, receivedMsg.Payload))
		} else {
			assert.Equal(t, tc.payload, receivedMsg.Payload, fmt.Sprintf("%s: expected %+v got %+v\n", tc.desc, tc.payload, receivedMsg.Payload))
		}
	}

	token = client.Unsubscribe(topic)
	token.Wait()
	require.Nil(t, token.Error(), fmt.Sprintf("got unexpected error: %s", token.Error()))
	token = client.Unsubscribe(fmt.Sprintf("%s.%s", topic, subtopic))
	token.Wait()
	require.Nil(t, token.Error(), fmt.Sprintf("got unexpected error: %s", token.Error()))
}

// Tests only the Subscriber
func TestSubscribe(t *testing.T) {
	cases := []struct {
		desc     string
		topic    string
		clientID string
		err      error
		handler  messaging.MessageHandler
	}{
		{
			desc:     "Subscribe to a topic with an ID",
			topic:    topic,
			clientID: "clientid1",
			err:      nil,
			handler:  handler{false},
		},
		{
			desc:     "Subscribe to the same topic with a different ID",
			topic:    topic,
			clientID: "clientid2",
			err:      nil,
			handler:  handler{false},
		},
		{
			desc:     "Subscribe to an already subscribed topic with an ID",
			topic:    topic,
			clientID: "clientid1",
			err:      nil,
			handler:  handler{false},
		},
		{
			desc:     "Subscribe to a topic with a subtopic with an ID",
			topic:    fmt.Sprintf("%s.%s", topic, subtopic),
			clientID: "clientid1",
			err:      nil,
			handler:  handler{false},
		},
		{
			desc:     "Subscribe to an already subscribed topic with a subtopic with an ID",
			topic:    fmt.Sprintf("%s.%s", topic, subtopic),
			clientID: "clientid1",
			err:      nil,
			handler:  handler{false},
		},
		{
			desc:     "Subscribe to an empty topic with an ID",
			topic:    "",
			clientID: "clientid1",
			err:      mqtt_pubsub.ErrEmptyTopic,
			handler:  handler{false},
		},
		{
			desc:     "Subscribe to a topic with empty id",
			topic:    topic,
			clientID: "",
			err:      mqtt_pubsub.ErrEmptyID,
			handler:  handler{false},
		},
	}
	for _, tc := range cases {
		err := pubsub.Subscribe(tc.clientID, tc.topic, tc.handler)
		switch tc.err {
		case nil:
			assert.Nil(t, err, "%s: got unexpected error: %s", tc.desc, err)
		default:
			assert.Equal(t, err, tc.err, fmt.Sprintf("%s: expected: %s, but got: %s", tc.desc, err, tc.err))
		}
	}
}

// Tests only the unsubscriber
func TestUnsubscribe(t *testing.T) {
	client, err := newClient(address, "clientID2", 30*time.Second)
	t.Cleanup(func() {
		defer client.Unsubscribe()
		defer client.Disconnect(5)
	})
	require.Nil(t, err, fmt.Sprintf("got unexpected error while creating client: %s", err))

	subs := []struct {
		desc     string
		topic    string
		clientID string
		err      error
		handler  messaging.MessageHandler
	}{
		{
			desc:     "Subscribe to a topic with an ID",
			topic:    topic,
			clientID: "clientid3",
			err:      nil,
			handler:  handler{false},
		},
		{
			desc:     "Subscribe to the same topic with a different ID",
			topic:    topic,
			clientID: "clientid4",
			err:      nil,
			handler:  handler{false},
		},
		{
			desc:     "Subscribe to a topic with a subtopic with an ID",
			topic:    fmt.Sprintf("%s.%s", topic, subtopic),
			clientID: "clientid3",
			err:      nil,
			handler:  handler{false},
		},
		{
			desc:     "Subscribe to an already subscribed topic with a subtopic with a different ID",
			topic:    fmt.Sprintf("%s.%s", topic, subtopic),
			clientID: "clientid4",
			err:      nil,
			handler:  handler{false},
		},
		{
			desc:     "Subscribe to a topic with an ID set to fail",
			topic:    fmt.Sprintf("%s%s", topic, "2"),
			clientID: "clientid5",
			err:      nil,
			handler:  handler{true},
		},
		{
			desc:     "Subscribe to a topic with a subtopic with an ID set to fail",
			topic:    fmt.Sprintf("%s%s.%s", topic, "3", subtopic),
			clientID: "clientid5",
			err:      nil,
			handler:  handler{true},
		},
	}

	for _, sub := range subs {
		token := client.Subscribe(sub.topic, qos, mqttHandler(handler{false}))
		assert.Nil(t, token.Error(), fmt.Sprintf("got unexpected error: %s", err))
	}

	cases := []struct {
		desc     string
		topic    string
		clientID string
		err      error
	}{
		{
			desc:     "Unsubscribe from a topic with an ID",
			topic:    topic,
			clientID: "clientid3",
			err:      nil,
		},
		{
			desc:     "Unsubscribe from a non-existent topic with an ID",
			topic:    "h",
			clientID: "clientid3",
			err:      mqtt_pubsub.ErrNotSubscribed,
		},
		{
			desc:     "Unsubscribe from the same topic with a different ID not subscribed",
			topic:    topic,
			clientID: "clientidd",
			err:      mqtt_pubsub.ErrNotSubscribed,
		},
		{
			desc:     "Unsubscribe from an already unsubscribed topic with an ID",
			topic:    topic,
			clientID: "clientid3",
			err:      mqtt_pubsub.ErrNotSubscribed,
		},
		{
			desc:     "Unsubscribe from a topic with a subtopic with an ID",
			topic:    topic,
			clientID: "clientid3",
			err:      nil,
		},
		{
			desc:     "Unsubscribe from an already unsubscribed topic with a subtopic with an ID",
			topic:    fmt.Sprintf("%s.%s", topic, subtopic),
			clientID: "clientid3",
			err:      mqtt_pubsub.ErrNotSubscribed,
		},
		{
			desc:     "Unsubscribe from an empty topic with an ID",
			topic:    "",
			clientID: "clientid3",
			err:      mqtt_pubsub.ErrEmptyTopic,
		},
		{
			desc:     "Unsubscribe from a topic with empty id",
			topic:    topic,
			clientID: "",
			err:      mqtt_pubsub.ErrEmptyID,
		},
		{
			desc:     "Unsubscribe from a topic with an ID with failing handler",
			topic:    fmt.Sprintf("%s%s", topic, "2"),
			clientID: "clientid5",
			err:      mqtt_pubsub.ErrFailedHandleMessage,
		},
		{
			desc:     "Unsubscribe from a topic with an ID with failing handler",
			topic:    fmt.Sprintf("%s%s.%s", topic, "2", subtopic),
			clientID: "clientid5",
			err:      mqtt_pubsub.ErrFailedHandleMessage,
		},
	}
	for _, tc := range cases {
		err := pubsub.Unsubscribe(tc.clientID, tc.topic)

		switch tc.err {
		case nil:
			if tc.err != nil {
				t.Errorf("%s: expected an error: %s, did not receive any", tc.desc, tc.err)
			}
		default:
			switch tc.err {
			case nil:
				t.Errorf("%s: got unexpected error: %s", tc.desc, err)
			default:
				assert.Equal(t, err.Error(), err.Error(), fmt.Sprintf("%s: expected: %s, but got: %s", tc.desc, err, tc.err))
			}
		}
	}
}

// Tests both publisher and subscriber
func TestPubSub(t *testing.T) {
	client, err := newClient(address, "", 30*time.Second)
	t.Cleanup(func() {
		defer client.Unsubscribe()
		defer client.Disconnect(5)
	})
	require.Nil(t, err, fmt.Sprintf("got unexpected error while creating client: %s", err))

	cases := []struct {
		desc     string
		topic    string
		clientID string
		err      error
		handler  messaging.MessageHandler
	}{
		{
			desc:     "Subscribe to a topic with an ID",
			topic:    topic,
			clientID: "clientid7",
			err:      nil,
			handler:  handler{false},
		},
		{
			desc:     "Subscribe to the same topic with a different ID",
			topic:    topic,
			clientID: "clientid8",
			err:      nil,
			handler:  handler{false},
		},
		{
			desc:     "Subscribe to a topic with a subtopic with an ID",
			topic:    fmt.Sprintf("%s.%s", topic, subtopic),
			clientID: "clientid7",
			err:      nil,
			handler:  handler{false},
		},
		{
			desc:     "Subscribe to an empty topic with an ID",
			topic:    "",
			clientID: "clientid7",
			err:      mqtt_pubsub.ErrEmptyTopic,
			handler:  handler{false},
		},
		{
			desc:     "Subscribe to a topic with empty id",
			topic:    topic,
			clientID: "",
			err:      mqtt_pubsub.ErrEmptyID,
			handler:  handler{false},
		},
	}
	for _, tc := range cases {
		err := pubsub.Subscribe(tc.clientID, tc.topic, tc.handler)
		switch err {
		case nil:
			// if no error, publish message, and receive after subscribing
			expectedMsg := messaging.Message{
				Channel:  channel,
				Subtopic: subtopic,
				Payload:  data,
			}
			token := client.Publish(tc.topic, qos, false, expectedMsg.Payload)
			token.Wait()

			receivedMsg := <-msgChan
			assert.Equal(t, expectedMsg.Payload, receivedMsg.Payload, fmt.Sprintf("%s: expected %+v got %+v\n", tc.desc, expectedMsg, receivedMsg))
		default:
			switch tc.err {
			case nil:
				assert.Nil(t, err, "%s: got unexpected error: %s", tc.desc, err.Error())
			default:
				assert.Equal(t, err, tc.err, fmt.Sprintf("%s: expected: %s, but got: %s", tc.desc, err, tc.err))
			}
		}
	}
}

// Tests both Subscribe and Unsubscribe
func TestSubUnsub(t *testing.T) {

	cases := []struct {
		desc     string
		topic    string
		clientID string
		err      error
		pubsub   bool //true for subscribe and false for unsubscribe
		handler  messaging.MessageHandler
	}{
		{
			desc:     "Subscribe to a topic with an ID",
			topic:    fmt.Sprintf("%s.%s", chansPrefix, topic),
			clientID: "clientid4",
			err:      nil,
			pubsub:   true,
			handler:  handler{false},
		},
		{
			desc:     "Unsubscribe from a topic with an ID",
			topic:    fmt.Sprintf("%s.%s", chansPrefix, topic),
			clientID: "clientid4",
			err:      nil,
			pubsub:   false,
			handler:  handler{false},
		},
		{
			desc:     "Unsubscribe from a non-existent topic with an ID",
			topic:    "h",
			clientID: "clientid4",
			err:      mqtt_pubsub.ErrNotSubscribed,
			pubsub:   false,
			handler:  handler{false},
		},
		{
			desc:     "Unsubscribe from an already unsubscribed topic with an ID",
			topic:    fmt.Sprintf("%s.%s", chansPrefix, topic),
			clientID: "clientid4",
			err:      mqtt_pubsub.ErrNotSubscribed,
			pubsub:   false,
			handler:  handler{false},
		},
		{
			desc:     "Subscribe to a topic with a subtopic with an ID",
			topic:    fmt.Sprintf("%s.%s.%s", chansPrefix, topic, subtopic),
			clientID: "clientidd4",
			err:      nil,
			pubsub:   true,
			handler:  handler{false},
		},
		{
			desc:     "Unsubscribe from a topic with a subtopic with an ID",
			topic:    fmt.Sprintf("%s.%s.%s", chansPrefix, topic, subtopic),
			clientID: "clientidd4",
			err:      nil,
			pubsub:   false,
			handler:  handler{false},
		},
		{
			desc:     "Unsubscribe from an already unsubscribed topic with a subtopic with an ID",
			topic:    fmt.Sprintf("%s.%s.%s", chansPrefix, topic, subtopic),
			clientID: "clientid4",
			err:      mqtt_pubsub.ErrNotSubscribed,
			pubsub:   false,
			handler:  handler{false},
		},
		{
			desc:     "Subscribe to an empty topic with an ID",
			topic:    "",
			clientID: "clientid4",
			err:      mqtt_pubsub.ErrEmptyTopic,
			pubsub:   true,
			handler:  handler{false},
		},
		{
			desc:     "Unsubscribe from an empty topic with an ID",
			topic:    "",
			clientID: "clientid4",
			err:      mqtt_pubsub.ErrEmptyTopic,
			pubsub:   false,
			handler:  handler{false},
		},
		{
			desc:     "Subscribe to a topic with empty ID",
			topic:    fmt.Sprintf("%s.%s", chansPrefix, topic),
			clientID: "",
			err:      mqtt_pubsub.ErrEmptyID,
			pubsub:   true,
			handler:  handler{false},
		},
		{
			desc:     "Unsubscribe from a topic with empty ID",
			topic:    fmt.Sprintf("%s.%s", chansPrefix, topic),
			clientID: "",
			err:      mqtt_pubsub.ErrEmptyID,
			pubsub:   false,
			handler:  handler{false},
		},
	}
	for _, tc := range cases {

		switch tc.pubsub {

		case true:
			err := pubsub.Subscribe(tc.clientID, tc.topic, tc.handler)
			switch tc.err {
			case nil:
				assert.Nil(t, err, fmt.Sprintf("%s: got unexpected error: %s", tc.desc, err))
			default:
				assert.Equal(t, err, tc.err, fmt.Sprintf("%s: expected: %s, but got: %s", tc.desc, tc.err, err))
			}

		default:
			err := pubsub.Unsubscribe(tc.clientID, tc.topic)
			switch tc.err {
			case nil:
				assert.Nil(t, err, fmt.Sprintf("%s: got unexpected error: %s", tc.desc, err))
			default:
				assert.Equal(t, err, tc.err, fmt.Sprintf("%s: expected: %s, but got: %s", tc.desc, tc.err, err))
			}
		}
	}
}

type handler struct {
	fail bool
}

func (h handler) Handle(msg messaging.Message) error {
	msgChan <- msg
	return nil
}

func (h handler) Cancel() error {
	if h.fail {
		return mqtt_pubsub.ErrFailedHandleMessage
	}
	return nil
}
