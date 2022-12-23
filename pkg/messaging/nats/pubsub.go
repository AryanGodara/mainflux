// Copyright (c) Mainflux
// SPDX-License-Identifier: Apache-2.0

package nats

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"

	log "github.com/mainflux/mainflux/logger"
	"github.com/mainflux/mainflux/pkg/messaging"
	broker "github.com/nats-io/nats.go"
)

const chansPrefix = "channels"

var (
	// ErrNotSubscribed indicates that the topic is not subscribed to.
	ErrNotSubscribed = errors.New("not subscribed")

	// ErrEmptyTopic indicates the absence of topic.
	ErrEmptyTopic = errors.New("empty topic")

	// ErrEmptyID indicates the absence of ID.
	ErrEmptyID = errors.New("empty ID")
)

var _ messaging.PubSub = (*pubsub)(nil)

type subscription struct {
	*broker.Subscription
	cancel func() error
}

type pubsub struct {
	publisher
	logger        log.Logger
	mu            sync.Mutex
	queue         string
	subscriptions map[string]map[string]subscription
}

// NewPubSub returns NATS message publisher/subscriber.
// Parameter queue specifies the queue for the Subscribe method.
// If queue is specified (is not an empty string), Subscribe method
// will execute NATS QueueSubscribe which is conceptually different
// from ordinary subscribe. For more information, please take a look
// here: https://docs.nats.io/developing-with-nats/receiving/queues.
// If the queue is empty, Subscribe will be used.
func NewPubSub(url, queue string, logger log.Logger) (messaging.PubSub, error) {
	opts := broker.Options{
		Url:            url,
		AllowReconnect: false,
		// MaxReconnect:     10,
		// ReconnectWait:    200 * time.Millisecond,
		Timeout: 1 * time.Second,
		// ReconnectBufSize: 5 * 1024 * 1024,
		// PingInterval:     1 * time.Second,
		// MaxPingsOut:      5,
	}
	conn, err := opts.Connect()
	if err != nil {
		return nil, err
	}
	js, err := conn.JetStream(broker.PublishAsyncMaxPending((256)))
	if err != nil {
		return nil, err
	}

	ret := &pubsub{
		publisher: publisher{
			conn:    conn,
			jetConn: js,
		},
		queue:         queue,
		logger:        logger,
		subscriptions: make(map[string]map[string]subscription),
	}

	ret.Create(js, "mf-nats-stream")

	return ret, nil
}

func (ps *pubsub) Subscribe(id, topic string, handler messaging.MessageHandler) error {
	if id == "" {
		return ErrEmptyID
	}
	if topic == "" {
		return ErrEmptyTopic
	}

	ps.mu.Lock()
	// Check topic
	s, ok := ps.subscriptions[topic]
	if ok {
		// Check client ID
		if _, ok := s[id]; ok {
			// Unlocking, so that Unsubscribe() can access ps.subscriptions
			ps.mu.Unlock()
			if err := ps.Unsubscribe(id, topic); err != nil {
				return err
			}

			ps.mu.Lock()
			// value of s can be changed while ps.mu is unlocked
			s = ps.subscriptions[topic]
		}
	}
	defer ps.mu.Unlock()
	if s == nil {
		s = make(map[string]subscription)
		ps.subscriptions[topic] = s
	}

	nh := ps.natsHandler(handler)

	var sub *broker.Subscription
	var err error

	switch ps.queue {
	case "":
		// sub, err = ps.conn.Subscribe(topic, nh)
		sub, err = ps.jetConn.Subscribe(topic, nh)
		if err != nil {
			return err
		}
		msgLimit := int(1e7)
		bytesLimit := int(1e10)

		if err = sub.SetPendingLimits(msgLimit, bytesLimit); err != nil {
			return err
		}

	default:
		sub, err = ps.conn.QueueSubscribe(topic, ps.queue, nh)
		if err != nil {
			return err
		}
		msgLimit := int(1e7)
		bytesLimit := int(1e10)

		if err = sub.SetPendingLimits(msgLimit, bytesLimit); err != nil {
			return err
		}
	}

	s[id] = subscription{
		Subscription: sub,
		cancel:       handler.Cancel,
	}

	return nil
}

func (ps *pubsub) Unsubscribe(id, topic string) error {
	if id == "" {
		return ErrEmptyID
	}
	if topic == "" {
		return ErrEmptyTopic
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	// Check topic
	s, ok := ps.subscriptions[topic]
	if !ok {
		return ErrNotSubscribed
	}
	// Check topic ID
	current, ok := s[id]
	if !ok {
		return ErrNotSubscribed
	}
	if current.cancel != nil {
		if err := current.cancel(); err != nil {
			return err
		}
	}
	if err := current.Unsubscribe(); err != nil {
		return err
	}
	delete(s, id)
	if len(s) == 0 {
		delete(ps.subscriptions, topic)
	}
	return nil
}

func (ps *pubsub) natsHandler(h messaging.MessageHandler) broker.MsgHandler {
	return func(m *broker.Msg) {
		var msg messaging.Message
		if err := proto.Unmarshal(m.Data, &msg); err != nil {
			ps.logger.Warn(fmt.Sprintf("Failed to unmarshal received message: %s", err))
			return
		}
		if err := h.Handle(msg); err != nil {
			ps.logger.Warn(fmt.Sprintf("Failed to handle Mainflux message: %s", err))
		}
	}
}

// Create creates the named stream.
func (ps *pubsub) Create(js broker.JetStreamContext, name string) *broker.StreamInfo {
	fmt.Printf("Creating stream: %q\n", name)
	strInfo, err := js.AddStream(&broker.StreamConfig{
		Name:     name,
		Subjects: []string{},
		MaxAge:   0, // 0 means keep forever
		Storage:  broker.FileStorage,
	})
	if err != nil {
		ps.logger.Warn(fmt.Sprintf("Could not create named stream : %v", err))
	}

	return strInfo
}

// Delete deletes the named stream.
func (ps *pubsub) Delete(js broker.JetStreamContext, name string) {
	fmt.Printf("Deleting stream: %q\n", name)
	if err := js.DeleteStream(name); err != nil {
		ps.logger.Warn(fmt.Sprintf("error deleting stream: %v", err))
	}
}

func (ps *pubsub) AddConsumer(js broker.JetStreamContext, strName, consName, consFilter string) {
	info, err := js.AddConsumer(strName, &broker.ConsumerConfig{
		Durable:   consName,
		AckPolicy: broker.AckExplicitPolicy,
		// MaxAckPending: 1,      // default value is 20,000
		FilterSubject: consFilter,
	})
	if err != nil {
		ps.logger.Warn(fmt.Sprintf("could not add consumer: %v", err))
	}
	ps.logger.Info(fmt.Sprintf("%v", info))
}
