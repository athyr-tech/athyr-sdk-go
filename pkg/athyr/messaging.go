package athyr

import (
	"context"

	athyr "github.com/athyr-tech/athyr-sdk-go/api/v1"
)

// Publish sends a message to the given subject.
func (c *agent) Publish(ctx context.Context, subject string, data []byte) error {
	if err := c.checkConnected(); err != nil {
		return err
	}

	_, err := c.athyr.Publish(ctx, &athyr.PublishRequest{
		AgentId: c.agentID,
		Subject: subject,
		Data:    data,
	})
	return err
}

// Subscribe creates a subscription to the given subject.
func (c *agent) Subscribe(ctx context.Context, subject string, handler MessageHandler) (Subscription, error) {
	return c.subscribe(ctx, subject, "", handler)
}

// QueueSubscribe creates a queue subscription for load balancing.
func (c *agent) QueueSubscribe(ctx context.Context, subject, queue string, handler MessageHandler) (Subscription, error) {
	return c.subscribe(ctx, subject, queue, handler)
}

func (c *agent) subscribe(ctx context.Context, subject, queue string, handler MessageHandler) (Subscription, error) {
	if err := c.checkConnected(); err != nil {
		return nil, err
	}

	stream, err := c.athyr.Subscribe(ctx, &athyr.SubscribeRequest{
		AgentId:    c.agentID,
		Subject:    subject,
		QueueGroup: queue,
	})
	if err != nil {
		return nil, err
	}

	sub := &subscription{
		cancel: func() { stream.CloseSend() },
	}

	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				return
			}
			handler(SubscribeMessage{
				Subject: msg.Subject,
				Data:    msg.Data,
				Reply:   msg.Reply,
			})
		}
	}()

	return sub, nil
}

// Request sends a request and waits for a response.
func (c *agent) Request(ctx context.Context, subject string, data []byte) ([]byte, error) {
	if err := c.checkConnected(); err != nil {
		return nil, err
	}

	resp, err := c.athyr.Request(ctx, &athyr.RequestMessage{
		AgentId:   c.agentID,
		Subject:   subject,
		Data:      data,
		TimeoutMs: c.opts.requestTimeout.Milliseconds(),
	})
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// subscription implements Subscription.
type subscription struct {
	cancel func()
}

func (s *subscription) Unsubscribe() error {
	s.cancel()
	return nil
}
