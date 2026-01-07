package athyr

import (
	"context"

	athyr "github.com/athyr-tech/athyr-sdk-go/api/v1"
)

// CreateSession creates a new memory session with the given profile and system prompt.
func (c *agent) CreateSession(ctx context.Context, profile SessionProfile, systemPrompt string) (*Session, error) {
	if err := c.checkConnected(); err != nil {
		return nil, err
	}

	resp, err := c.athyr.CreateSession(ctx, &athyr.CreateSessionRequest{
		AgentId: c.agentID,
		Profile: &athyr.SessionProfile{
			Type:                   profile.Type,
			MaxTokens:              int32(profile.MaxTokens),
			SummarizationThreshold: int32(profile.SummarizationThreshold),
		},
		SystemPrompt: systemPrompt,
	})
	if err != nil {
		return nil, wrapError("CreateSession", err)
	}

	return protoToSession(resp), nil
}

// GetSession retrieves a session by ID.
func (c *agent) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	if err := c.checkConnected(); err != nil {
		return nil, err
	}

	resp, err := c.athyr.GetSession(ctx, &athyr.GetSessionRequest{
		AgentId:   c.agentID,
		SessionId: sessionID,
	})
	if err != nil {
		return nil, wrapError("GetSession", err)
	}

	return protoToSession(resp), nil
}

// DeleteSession removes a session.
func (c *agent) DeleteSession(ctx context.Context, sessionID string) error {
	if err := c.checkConnected(); err != nil {
		return err
	}

	_, err := c.athyr.DeleteSession(ctx, &athyr.DeleteSessionRequest{
		AgentId:   c.agentID,
		SessionId: sessionID,
	})
	return wrapError("DeleteSession", err)
}

// AddHint adds a persistent hint to a session.
func (c *agent) AddHint(ctx context.Context, sessionID, hint string) error {
	if err := c.checkConnected(); err != nil {
		return err
	}

	_, err := c.athyr.AddHint(ctx, &athyr.AddHintRequest{
		AgentId:   c.agentID,
		SessionId: sessionID,
		Hint:      hint,
	})
	return wrapError("AddHint", err)
}

// protoToSession converts a proto Session to SDK Session.
func protoToSession(s *athyr.Session) *Session {
	session := &Session{
		ID:           s.Id,
		AgentID:      s.AgentId,
		SystemPrompt: s.SystemPrompt,
		Summary:      s.Summary,
		Hints:        s.Hints,
	}

	if s.CreatedAt != nil {
		session.CreatedAt = s.CreatedAt.AsTime()
	}
	if s.UpdatedAt != nil {
		session.UpdatedAt = s.UpdatedAt.AsTime()
	}
	if s.Profile != nil {
		session.Profile = SessionProfile{
			Type:                   s.Profile.Type,
			MaxTokens:              int(s.Profile.MaxTokens),
			SummarizationThreshold: int(s.Profile.SummarizationThreshold),
		}
	}

	for _, msg := range s.Messages {
		m := SessionMessage{
			Role:    msg.Role,
			Content: msg.Content,
			Tokens:  int(msg.Tokens),
		}
		if msg.Timestamp != nil {
			m.Timestamp = msg.Timestamp.AsTime()
		}
		session.Messages = append(session.Messages, m)
	}

	return session
}
