package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	sdk "github.com/athyr-tech/athyr-sdk-go/pkg/agent"
)

// GroupChat orchestrates multi-agent collaborative discussions where
// participants take turns contributing to a shared conversation.
//
// Example:
//
//	chat := NewGroupChat("product-review").
//	    Manager(RoundRobinManager()).
//	    Participant("pm", "agent.pm.invoke").
//	    Participant("engineer", "agent.engineer.invoke").
//	    Participant("designer", "agent.designer.invoke").
//	    MaxRounds(10).
//	    ConsensusCheck(func(msgs []Message) bool {
//	        // Check if participants reached agreement
//	        return detectAgreement(msgs)
//	    })
//
//	result, err := chat.Discuss(ctx, client, topic)
type GroupChat struct {
	name         string
	manager      Manager
	participants []participant
	maxRounds    int
	consensus    ConsensusFunc
}

type participant struct {
	name    string
	subject string
}

// NewGroupChat creates a new group chat orchestrator.
func NewGroupChat(name string) *GroupChat {
	return &GroupChat{
		name:         name,
		participants: make([]participant, 0),
		maxRounds:    10, // sensible default
		manager:      RoundRobinManager(),
	}
}

// Manager sets the selection strategy for choosing the next speaker.
func (g *GroupChat) Manager(m Manager) *GroupChat {
	g.manager = m
	return g
}

// Participant adds a participant to the group chat.
func (g *GroupChat) Participant(name, subject string) *GroupChat {
	g.participants = append(g.participants, participant{
		name:    name,
		subject: subject,
	})
	return g
}

// MaxRounds sets the maximum number of discussion rounds.
// Each round allows one participant to speak.
// Default is 10.
func (g *GroupChat) MaxRounds(n int) *GroupChat {
	g.maxRounds = n
	return g
}

// ConsensusFunc checks if the group has reached consensus.
// Returns true to end the discussion early.
type ConsensusFunc func(messages []Message) bool

// ConsensusCheck sets a function to detect when consensus is reached.
// If the function returns true, the discussion ends early.
func (g *GroupChat) ConsensusCheck(fn ConsensusFunc) *GroupChat {
	g.consensus = fn
	return g
}

// Message represents a single contribution to the discussion.
type Message struct {
	Speaker   string    `json:"speaker"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// ChatRequest is the input format for group chat participants.
// Participants receive the topic and full conversation history.
type ChatRequest struct {
	Topic    string    `json:"topic"`
	Messages []Message `json:"messages"`
	Round    int       `json:"round"`
}

// ChatResponse is the expected output format from participants.
type ChatResponse struct {
	Content string `json:"content"`
}

// Discuss starts a group discussion on the given topic.
// Returns the conclusion (last message) when discussion ends.
func (g *GroupChat) Discuss(ctx context.Context, client sdk.Agent, topic []byte) ([]byte, error) {
	trace, err := g.DiscussWithTrace(ctx, client, topic)
	if err != nil {
		return nil, err
	}
	return trace.Conclusion, nil
}

// DiscussWithTrace runs the discussion and returns detailed trace.
func (g *GroupChat) DiscussWithTrace(ctx context.Context, client sdk.Agent, topic []byte) (*ChatTrace, error) {
	if len(g.participants) == 0 {
		return nil, &ChatError{Err: fmt.Errorf("no participants configured")}
	}

	trace := &ChatTrace{
		GroupChat:    g.name,
		Topic:        string(topic),
		Messages:     make([]Message, 0),
		StartedAt:    time.Now(),
		Participants: make([]string, 0, len(g.participants)),
	}

	for _, p := range g.participants {
		trace.Participants = append(trace.Participants, p.name)
	}

	// Initialize manager state
	state := g.manager.Init(trace.Participants)

	// Build participant lookup
	participantMap := make(map[string]participant)
	for _, p := range g.participants {
		participantMap[p.name] = p
	}

	for round := 0; round < g.maxRounds; round++ {
		// Select next speaker
		speaker := g.manager.Next(state, trace.Messages)
		if speaker == "" {
			// No more speakers (manager signals done)
			break
		}

		p, ok := participantMap[speaker]
		if !ok {
			trace.Duration = time.Since(trace.StartedAt)
			trace.Error = &ChatError{
				Participant: speaker,
				Err:         fmt.Errorf("unknown participant %q selected by manager", speaker),
			}
			return trace, trace.Error
		}

		// Prepare request for participant
		req := ChatRequest{
			Topic:    string(topic),
			Messages: trace.Messages,
			Round:    round,
		}
		reqBytes, err := json.Marshal(req)
		if err != nil {
			trace.Duration = time.Since(trace.StartedAt)
			trace.Error = &ChatError{Err: fmt.Errorf("failed to marshal request: %w", err)}
			return trace, trace.Error
		}

		// Call participant
		respBytes, err := client.Request(ctx, p.subject, reqBytes)
		if err != nil {
			trace.Duration = time.Since(trace.StartedAt)
			trace.Error = &ChatError{
				Participant: p.name,
				Round:       round,
				Err:         err,
			}
			return trace, trace.Error
		}

		// Parse response
		var resp ChatResponse
		if err := json.Unmarshal(respBytes, &resp); err != nil {
			// If not JSON, treat raw response as content
			resp.Content = string(respBytes)
		}

		// Add message to history
		msg := Message{
			Speaker:   p.name,
			Content:   resp.Content,
			Timestamp: time.Now(),
		}
		trace.Messages = append(trace.Messages, msg)

		// Check for consensus
		if g.consensus != nil && g.consensus(trace.Messages) {
			trace.ConsensusReached = true
			break
		}
	}

	trace.Duration = time.Since(trace.StartedAt)
	trace.RoundsCompleted = len(trace.Messages)

	// Set conclusion to last message content
	if len(trace.Messages) > 0 {
		lastMsg := trace.Messages[len(trace.Messages)-1]
		trace.Conclusion = []byte(lastMsg.Content)
	}

	return trace, nil
}

// ChatTrace contains detailed discussion information.
type ChatTrace struct {
	GroupChat        string
	Topic            string
	Messages         []Message
	Participants     []string
	RoundsCompleted  int
	ConsensusReached bool
	Conclusion       []byte
	StartedAt        time.Time
	Duration         time.Duration
	Error            *ChatError
}

// Transcript returns a formatted string of the discussion.
func (t *ChatTrace) Transcript() string {
	var result string
	for _, msg := range t.Messages {
		result += fmt.Sprintf("[%s]: %s\n", msg.Speaker, msg.Content)
	}
	return result
}

// ChatError indicates group chat failure.
type ChatError struct {
	Participant string // Which participant caused the error
	Round       int    // Which round failed
	Err         error
}

func (e *ChatError) Error() string {
	if e.Participant != "" {
		return fmt.Sprintf("group chat failed: participant %q in round %d: %v",
			e.Participant, e.Round, e.Err)
	}
	return fmt.Sprintf("group chat failed: %v", e.Err)
}

func (e *ChatError) Unwrap() error {
	return e.Err
}

// Manager selects which participant speaks next.
type Manager interface {
	// Init initializes manager state with participant list.
	// Returns an opaque state object passed to subsequent calls.
	Init(participants []string) interface{}

	// Next selects the next speaker based on current state and message history.
	// Returns empty string to signal discussion should end.
	Next(state interface{}, messages []Message) string
}

// roundRobinManager implements round-robin speaker selection.
type roundRobinManager struct{}

type roundRobinState struct {
	participants []string
	index        int
}

// RoundRobinManager creates a manager that selects speakers in order.
// Each participant speaks once per cycle, repeating until max rounds.
func RoundRobinManager() Manager {
	return &roundRobinManager{}
}

func (m *roundRobinManager) Init(participants []string) interface{} {
	return &roundRobinState{
		participants: participants,
		index:        0,
	}
}

func (m *roundRobinManager) Next(state interface{}, messages []Message) string {
	s := state.(*roundRobinState)
	if len(s.participants) == 0 {
		return ""
	}
	speaker := s.participants[s.index]
	s.index = (s.index + 1) % len(s.participants)
	return speaker
}

// funcManager wraps a custom selection function.
type funcManager struct {
	selectFn func(participants []string, messages []Message) string
}

type funcState struct {
	participants []string
}

// FuncManager creates a manager using a custom selection function.
// The function receives the participant list and message history,
// and returns the name of the next speaker (or empty to end).
func FuncManager(fn func(participants []string, messages []Message) string) Manager {
	return &funcManager{selectFn: fn}
}

func (m *funcManager) Init(participants []string) interface{} {
	return &funcState{participants: participants}
}

func (m *funcManager) Next(state interface{}, messages []Message) string {
	s := state.(*funcState)
	return m.selectFn(s.participants, messages)
}
