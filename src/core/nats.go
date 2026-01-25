package core

import (
	"time"

	"github.com/nats-io/nats.go"
)

// NatsClient wrap nats connection
type NatsClient struct {
	Conn *nats.Conn
}

// NewNatsClient creates a new NATS client
func NewNatsClient(url string) (*NatsClient, error) {
	nc, err := nats.Connect(url, nats.Timeout(5*time.Second))
	if err != nil {
		return nil, err
	}
	return &NatsClient{Conn: nc}, nil
}

// Publish publishes a message to a subject
func (n *NatsClient) Publish(subject string, data []byte) error {
	return n.Conn.Publish(subject, data)
}

// Subscribe subscribes to a subject
func (n *NatsClient) Subscribe(subject string, cb nats.MsgHandler) (*nats.Subscription, error) {
	return n.Conn.Subscribe(subject, cb)
}

// Close closes the connection
func (n *NatsClient) Close() {
	if n.Conn != nil {
		n.Conn.Close()
	}
}
