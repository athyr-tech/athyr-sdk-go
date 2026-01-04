package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestGroupChat_NoParticipants(t *testing.T) {
	client := newMockAgent()
	chat := NewGroupChat("empty")

	_, err := chat.Discuss(context.Background(), client, []byte("topic"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var chatErr *ChatError
	if !errors.As(err, &chatErr) {
		t.Fatalf("expected ChatError, got %T", err)
	}
}

func TestGroupChat_SingleParticipant(t *testing.T) {
	resp := ChatResponse{Content: "my contribution"}
	respBytes, _ := json.Marshal(resp)

	client := newMockAgent().
		OnRequest("agent.solo.invoke", respBytes)

	chat := NewGroupChat("solo-chat").
		Participant("solo", "agent.solo.invoke").
		MaxRounds(3)

	trace, err := chat.DiscussWithTrace(context.Background(), client, []byte("topic"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trace.Messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(trace.Messages))
	}

	// All messages from same participant
	for _, msg := range trace.Messages {
		if msg.Speaker != "solo" {
			t.Errorf("expected speaker 'solo', got '%s'", msg.Speaker)
		}
	}
}

func TestGroupChat_RoundRobin(t *testing.T) {
	respA := ChatResponse{Content: "A says hello"}
	respB := ChatResponse{Content: "B responds"}
	respC := ChatResponse{Content: "C adds"}
	respABytes, _ := json.Marshal(respA)
	respBBytes, _ := json.Marshal(respB)
	respCBytes, _ := json.Marshal(respC)

	client := newMockAgent().
		OnRequest("agent.a.invoke", respABytes).
		OnRequest("agent.b.invoke", respBBytes).
		OnRequest("agent.c.invoke", respCBytes)

	chat := NewGroupChat("round-robin").
		Manager(RoundRobinManager()).
		Participant("A", "agent.a.invoke").
		Participant("B", "agent.b.invoke").
		Participant("C", "agent.c.invoke").
		MaxRounds(6)

	trace, err := chat.DiscussWithTrace(context.Background(), client, []byte("discuss"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trace.Messages) != 6 {
		t.Fatalf("expected 6 messages, got %d", len(trace.Messages))
	}

	// Verify round-robin order: A, B, C, A, B, C
	expectedOrder := []string{"A", "B", "C", "A", "B", "C"}
	for i, expected := range expectedOrder {
		if trace.Messages[i].Speaker != expected {
			t.Errorf("message %d: expected speaker '%s', got '%s'",
				i, expected, trace.Messages[i].Speaker)
		}
	}
}

func TestGroupChat_ReceivesHistory(t *testing.T) {
	// Capture requests to verify history is passed
	var receivedRequests []ChatRequest

	client := newMockAgent()
	client.responses["agent.a.invoke"] = []byte(`{"content":"A response"}`)
	client.responses["agent.b.invoke"] = []byte(`{"content":"B response"}`)

	// Override to capture requests
	origRequest := client.Request
	_ = origRequest // silence unused warning

	chat := NewGroupChat("history-test").
		Participant("A", "agent.a.invoke").
		Participant("B", "agent.b.invoke").
		MaxRounds(4)

	trace, err := chat.DiscussWithTrace(context.Background(), client, []byte("test topic"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse requests from mock calls
	for _, call := range client.calls {
		var req ChatRequest
		if err := json.Unmarshal(call.data, &req); err == nil {
			receivedRequests = append(receivedRequests, req)
		}
	}

	// First request should have no history
	if len(receivedRequests) > 0 && len(receivedRequests[0].Messages) != 0 {
		t.Error("first request should have no message history")
	}

	// Second request should have 1 message
	if len(receivedRequests) > 1 && len(receivedRequests[1].Messages) != 1 {
		t.Errorf("second request should have 1 message, got %d", len(receivedRequests[1].Messages))
	}

	// Third request should have 2 messages
	if len(receivedRequests) > 2 && len(receivedRequests[2].Messages) != 2 {
		t.Errorf("third request should have 2 messages, got %d", len(receivedRequests[2].Messages))
	}

	if trace.Topic != "test topic" {
		t.Errorf("expected topic 'test topic', got '%s'", trace.Topic)
	}
}

func TestGroupChat_ConsensusDetection(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.a.invoke", []byte(`{"content":"I agree"}`)).
		OnRequest("agent.b.invoke", []byte(`{"content":"I agree too"}`))

	consensusReached := false
	chat := NewGroupChat("consensus-test").
		Participant("A", "agent.a.invoke").
		Participant("B", "agent.b.invoke").
		MaxRounds(10).
		ConsensusCheck(func(messages []Message) bool {
			// Detect consensus when both say "agree"
			agreeCount := 0
			for _, msg := range messages {
				if contains(msg.Content, "agree") {
					agreeCount++
				}
			}
			if agreeCount >= 2 {
				consensusReached = true
				return true
			}
			return false
		})

	trace, err := chat.DiscussWithTrace(context.Background(), client, []byte("proposal"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !trace.ConsensusReached {
		t.Error("expected consensus to be reached")
	}

	// Should have stopped early (not all 10 rounds)
	if len(trace.Messages) > 2 {
		t.Errorf("expected early termination at 2 messages, got %d", len(trace.Messages))
	}

	if !consensusReached {
		t.Error("consensus callback should have been triggered")
	}
}

func TestGroupChat_ParticipantError(t *testing.T) {
	expectedErr := errors.New("agent unavailable")
	client := newMockAgent().
		OnRequest("agent.a.invoke", []byte(`{"content":"A speaks"}`)).
		OnRequestError("agent.b.invoke", expectedErr)

	chat := NewGroupChat("error-test").
		Participant("A", "agent.a.invoke").
		Participant("B", "agent.b.invoke").
		MaxRounds(4)

	_, err := chat.Discuss(context.Background(), client, []byte("topic"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var chatErr *ChatError
	if !errors.As(err, &chatErr) {
		t.Fatalf("expected ChatError, got %T", err)
	}

	if chatErr.Participant != "B" {
		t.Errorf("expected participant 'B', got '%s'", chatErr.Participant)
	}

	if chatErr.Round != 1 {
		t.Errorf("expected round 1, got %d", chatErr.Round)
	}

	if !errors.Is(chatErr, expectedErr) {
		t.Error("expected underlying error to be wrapped")
	}
}

func TestGroupChat_Transcript(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.pm.invoke", []byte(`{"content":"We need feature X"}`)).
		OnRequest("agent.dev.invoke", []byte(`{"content":"That will take 2 weeks"}`))

	chat := NewGroupChat("transcript-test").
		Participant("PM", "agent.pm.invoke").
		Participant("Dev", "agent.dev.invoke").
		MaxRounds(2)

	trace, err := chat.DiscussWithTrace(context.Background(), client, []byte("planning"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	transcript := trace.Transcript()
	if !contains(transcript, "[PM]:") {
		t.Error("transcript should contain PM speaker")
	}
	if !contains(transcript, "[Dev]:") {
		t.Error("transcript should contain Dev speaker")
	}
	if !contains(transcript, "feature X") {
		t.Error("transcript should contain PM's message")
	}
}

func TestGroupChat_Conclusion(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.a.invoke", []byte(`{"content":"first"}`)).
		OnRequest("agent.b.invoke", []byte(`{"content":"final conclusion"}`))

	chat := NewGroupChat("conclusion-test").
		Participant("A", "agent.a.invoke").
		Participant("B", "agent.b.invoke").
		MaxRounds(2)

	result, err := chat.Discuss(context.Background(), client, []byte("topic"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Result should be last message content
	if string(result) != "final conclusion" {
		t.Errorf("expected 'final conclusion', got '%s'", result)
	}
}

func TestGroupChat_NonJSONResponse(t *testing.T) {
	// Test that non-JSON responses are handled gracefully
	client := newMockAgent().
		OnRequest("agent.simple.invoke", []byte("plain text response"))

	chat := NewGroupChat("plain-text").
		Participant("simple", "agent.simple.invoke").
		MaxRounds(1)

	trace, err := chat.DiscussWithTrace(context.Background(), client, []byte("topic"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trace.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(trace.Messages))
	}

	if trace.Messages[0].Content != "plain text response" {
		t.Errorf("expected 'plain text response', got '%s'", trace.Messages[0].Content)
	}
}

func TestGroupChat_FuncManager(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.a.invoke", []byte(`{"content":"A"}`)).
		OnRequest("agent.b.invoke", []byte(`{"content":"B"}`))

	// Custom manager: always select B after first round
	customManager := FuncManager(func(participants []string, messages []Message) string {
		if len(messages) == 0 {
			return "A" // First speaker
		}
		return "B" // Always B after that
	})

	chat := NewGroupChat("custom-manager").
		Manager(customManager).
		Participant("A", "agent.a.invoke").
		Participant("B", "agent.b.invoke").
		MaxRounds(4)

	trace, err := chat.DiscussWithTrace(context.Background(), client, []byte("topic"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be: A, B, B, B
	expectedOrder := []string{"A", "B", "B", "B"}
	for i, expected := range expectedOrder {
		if trace.Messages[i].Speaker != expected {
			t.Errorf("message %d: expected '%s', got '%s'",
				i, expected, trace.Messages[i].Speaker)
		}
	}
}

func TestGroupChat_FuncManagerEarlyStop(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.a.invoke", []byte(`{"content":"done"}`))

	// Custom manager that stops after one message
	stopManager := FuncManager(func(participants []string, messages []Message) string {
		if len(messages) > 0 {
			return "" // Empty string signals stop
		}
		return participants[0]
	})

	chat := NewGroupChat("early-stop").
		Manager(stopManager).
		Participant("A", "agent.a.invoke").
		Participant("B", "agent.b.invoke").
		MaxRounds(10)

	trace, err := chat.DiscussWithTrace(context.Background(), client, []byte("topic"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should stop after 1 message despite 10 max rounds
	if len(trace.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(trace.Messages))
	}
}

func TestGroupChat_Reusable(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.a.invoke", []byte(`{"content":"response"}`))

	chat := NewGroupChat("reusable").
		Participant("A", "agent.a.invoke").
		MaxRounds(2)

	// Execute twice
	_, err1 := chat.Discuss(context.Background(), client, []byte("topic1"))
	_, err2 := chat.Discuss(context.Background(), client, []byte("topic2"))

	if err1 != nil || err2 != nil {
		t.Error("chat should be reusable")
	}

	if len(client.calls) != 4 { // 2 rounds * 2 executions
		t.Errorf("expected 4 calls, got %d", len(client.calls))
	}
}

func TestChatError_Error(t *testing.T) {
	// With participant and round
	err1 := &ChatError{
		Participant: "A",
		Round:       3,
		Err:         errors.New("timeout"),
	}
	expected1 := `group chat failed: participant "A" in round 3: timeout`
	if err1.Error() != expected1 {
		t.Errorf("expected '%s', got '%s'", expected1, err1.Error())
	}

	// Without participant
	err2 := &ChatError{
		Err: errors.New("no participants"),
	}
	expected2 := "group chat failed: no participants"
	if err2.Error() != expected2 {
		t.Errorf("expected '%s', got '%s'", expected2, err2.Error())
	}
}

func TestChatError_Unwrap(t *testing.T) {
	underlying := errors.New("underlying")
	err := &ChatError{Err: underlying}

	if !errors.Is(err, underlying) {
		t.Error("expected Unwrap to return underlying error")
	}
}

func TestGroupChat_Participants(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.a.invoke", []byte(`{"content":"A"}`)).
		OnRequest("agent.b.invoke", []byte(`{"content":"B"}`))

	chat := NewGroupChat("participants-test").
		Participant("Alice", "agent.a.invoke").
		Participant("Bob", "agent.b.invoke").
		MaxRounds(2)

	trace, err := chat.DiscussWithTrace(context.Background(), client, []byte("topic"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trace.Participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(trace.Participants))
	}

	if trace.Participants[0] != "Alice" || trace.Participants[1] != "Bob" {
		t.Errorf("unexpected participants: %v", trace.Participants)
	}
}

func TestGroupChat_RoundsCompleted(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.a.invoke", []byte(`{"content":"done"}`))

	chat := NewGroupChat("rounds-test").
		Participant("A", "agent.a.invoke").
		MaxRounds(5).
		ConsensusCheck(func(messages []Message) bool {
			return len(messages) >= 3
		})

	trace, err := chat.DiscussWithTrace(context.Background(), client, []byte("topic"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if trace.RoundsCompleted != 3 {
		t.Errorf("expected 3 rounds completed, got %d", trace.RoundsCompleted)
	}
}
