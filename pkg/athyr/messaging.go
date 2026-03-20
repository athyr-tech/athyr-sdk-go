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
	return wrapError("Publish", err)
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
		return nil, wrapError("Subscribe", err)
	}

	sub := &subscription{
		agent:   c,
		subject: subject,
		queue:   queue,
		handler: handler,
		cancel:  func() { stream.CloseSend() },
	}

	// Track subscription for reconnection recovery
	c.trackSubscription(subject, queue, handler)

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

	// Enforce requestTimeout client-side if no deadline already set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.opts.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.opts.requestTimeout)
		defer cancel()
	}

	resp, err := c.athyr.Request(ctx, &athyr.RequestMessage{
		AgentId:   c.agentID,
		Subject:   subject,
		Data:      data,
		TimeoutMs: c.opts.requestTimeout.Milliseconds(),
	})
	if err != nil {
		return nil, wrapError("Request", err)
	}

	return resp.Data, nil
}

// subscription implements Subscription.
type subscription struct {
	agent   *agent
	subject string
	queue   string
	handler MessageHandler
	cancel  func()
}

func (s *subscription) Unsubscribe() error {
	s.cancel()
	// Remove from tracked subscriptions
	if s.agent != nil {
		s.agent.untrackSubscription(s.subject, s.queue)
	}
	return nil
}

// trackSubscription adds a subscription to the tracking list for reconnection recovery.
func (c *agent) trackSubscription(subject, queue string, handler MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Avoid duplicates (can happen during resubscription)
	for _, sub := range c.subscriptions {
		if sub.subject == subject && sub.queue == queue {
			return
		}
	}

	c.subscriptions = append(c.subscriptions, subRecord{
		subject: subject,
		queue:   queue,
		handler: handler,
	})
}

// untrackSubscription removes a subscription from the tracking list.
func (c *agent) untrackSubscription(subject, queue string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, sub := range c.subscriptions {
		if sub.subject == subject && sub.queue == queue {
			// Remove by swapping with last element and truncating
			c.subscriptions[i] = c.subscriptions[len(c.subscriptions)-1]
			c.subscriptions = c.subscriptions[:len(c.subscriptions)-1]
			return
		}
	}
}
