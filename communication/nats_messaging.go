package communication

import (
	"fmt"

	"github.com/NethermindEth/chaoschain-launchpad/core"
	"github.com/nats-io/nats.go"
)

// Messenger encapsulates a NATS broker connection.
type Messenger struct {
	broker *core.NATSBroker
}

// NewMessenger creates a new instance of Messenger.
func NewMessenger(url string) (*Messenger, error) {
	broker, err := core.NewNATSBroker(url)
	if err != nil {
		return nil, err
	}
	return &Messenger{broker: broker}, nil
}

// PublishGlobal publishes a message to a global subject (for public announcements).
func (m *Messenger) PublishGlobal(subject, message string) error {
	return m.broker.Publish(subject, []byte(message))
}

// PublishPrivate sends a message directly to a specific agent by using a private subject.
func (m *Messenger) PublishPrivate(agentID, message string) error {
	subject := fmt.Sprintf("agent.%s.private", agentID)
	return m.broker.Publish(subject, []byte(message))
}

// SubscribeGlobal subscribes to a global topic.
func (m *Messenger) SubscribeGlobal(subject string, handler nats.MsgHandler) error {
	return m.broker.Subscribe(subject, handler)
}

// SubscribePrivate subscribes to private messages for an agent.
func (m *Messenger) SubscribePrivate(agentID string, handler nats.MsgHandler) error {
	subject := fmt.Sprintf("agent.%s.private", agentID)
	return m.broker.Subscribe(subject, handler)
}
