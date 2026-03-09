package athyr

import (
	"context"

	athyr "github.com/athyr-tech/athyr-sdk-go/api/v1"
)

// ListAgents returns all registered agents on the platform.
// Pass a non-empty skillFilter to only return agents with that capability.
func (c *agent) ListAgents(ctx context.Context, skillFilter string) ([]AgentInfo, error) {
	if err := c.checkConnected(); err != nil {
		return nil, err
	}

	resp, err := c.athyr.ListAgents(ctx, &athyr.ListAgentsRequest{
		SkillFilter: skillFilter,
	})
	if err != nil {
		return nil, wrapError("ListAgents", err)
	}

	agents := make([]AgentInfo, len(resp.Agents))
	for i, a := range resp.Agents {
		agents[i] = protoToAgentInfo(a)
	}

	return agents, nil
}

// GetAgent retrieves a specific agent by ID.
func (c *agent) GetAgent(ctx context.Context, agentID string) (*AgentInfo, error) {
	if err := c.checkConnected(); err != nil {
		return nil, err
	}

	resp, err := c.athyr.GetAgent(ctx, &athyr.GetAgentRequest{
		AgentId: agentID,
	})
	if err != nil {
		return nil, wrapError("GetAgent", err)
	}

	info := protoToAgentInfo(resp)
	return &info, nil
}

// protoToAgentInfo converts a proto AgentInfo to SDK AgentInfo.
func protoToAgentInfo(a *athyr.AgentInfo) AgentInfo {
	info := AgentInfo{
		ID:     a.Id,
		Status: a.Status,
	}

	if a.Card != nil {
		info.Card = AgentCard{
			Name:         a.Card.Name,
			Description:  a.Card.Description,
			Version:      a.Card.Version,
			Capabilities: a.Card.Capabilities,
			Metadata:     a.Card.Metadata,
		}
	}

	if a.ConnectedAt != nil {
		info.ConnectedAt = a.ConnectedAt.AsTime()
	}
	if a.LastSeen != nil {
		info.LastSeen = a.LastSeen.AsTime()
	}

	return info
}
