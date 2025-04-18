package events

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type NatsConnector struct {
	Connection *nats.Conn
	Stream     jetstream.Stream
}

type EventMessage interface {
	Subject() string
	Data() []byte
	Ack() error
}

func NewNatsConnector(url string, options []nats.Option) (*NatsConnector, error) {
	nc, err := nats.Connect(url, options...)
	if err != nil {
		return nil, err
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()

		return nil, err
	}

	config := jetstream.StreamConfig{
		Name:      "EVENTS",
		Retention: jetstream.WorkQueuePolicy,
		Subjects:  []string{"events.>"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := js.CreateStream(ctx, config)
	if err != nil {
		nc.Close()
		return nil, err
	}

	return &NatsConnector{Connection: nc, Stream: stream}, nil
}

func (nc *NatsConnector) IsConnected() bool {

	fmt.Println(nc.Connection.IsReconnecting())
	fmt.Println(nc.Connection.Status().String())
	fmt.Println((nc.Connection.IsConnected()))
	return nc.Connection.Status() == nats.CONNECTED
}

func (nc *NatsConnector) Publish(ctx context.Context, subject string, data []byte) error {

	js, err := jetstream.New(nc.Connection)
	if err != nil {
		nc.Close()
		return err
	}

	status := js.Conn().Status()
	if status != nats.CONNECTED {
		return nats.ErrConnectionClosed
	}

	_, err = js.Publish(ctx, subject, data)

	if err != nil {
		return err
	}

	return nil
}

func (nc *NatsConnector) Subscribe(ctx context.Context, subscriptionName string, subject string, handler jetstream.MessageHandler) error {
	consumer, err := nc.Stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		FilterSubject: subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		Durable:       subscriptionName,
	})

	if err != nil {
		return err
	}

	_, err = consumer.Consume(handler)
	if err != nil {
		return err
	}

	return nil
}

func (nc *NatsConnector) Close() {
	nc.Connection.Close()
}
