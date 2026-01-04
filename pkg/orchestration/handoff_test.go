package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestHandoffRouter_NoTriage(t *testing.T) {
	client := newMockAgent()
	router := NewHandoffRouter("test")

	_, err := router.Handle(context.Background(), client, []byte("input"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var handoffErr *HandoffError
	if !errors.As(err, &handoffErr) {
		t.Fatalf("expected HandoffError, got %T", err)
	}
}

func TestHandoffRouter_TriageHandlesDirect(t *testing.T) {
	// Triage handles the request directly without routing
	decision := HandoffDecision{
		Handled:  true,
		Response: json.RawMessage(`"handled by triage"`),
	}
	decisionBytes, _ := json.Marshal(decision)

	client := newMockAgent().
		OnRequest("agent.triage.invoke", decisionBytes)

	router := NewHandoffRouter("support").
		Triage("agent.triage.invoke").
		Route("billing", "agent.billing.invoke")

	result, err := router.Handle(context.Background(), client, []byte("simple question"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result) != `"handled by triage"` {
		t.Errorf("expected '\"handled by triage\"', got '%s'", result)
	}
}

func TestHandoffRouter_SingleHandoff(t *testing.T) {
	// Triage routes to billing, billing returns final response
	triageDecision := HandoffDecision{
		Route: "billing",
	}
	triageBytes, _ := json.Marshal(triageDecision)

	client := newMockAgent().
		OnRequest("agent.triage.invoke", triageBytes).
		OnRequest("agent.billing.invoke", []byte(`{"status":"resolved"}`))

	router := NewHandoffRouter("support").
		Triage("agent.triage.invoke").
		Route("billing", "agent.billing.invoke").
		Route("technical", "agent.technical.invoke")

	result, err := router.Handle(context.Background(), client, []byte("billing issue"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result) != `{"status":"resolved"}` {
		t.Errorf("expected resolved status, got '%s'", result)
	}
}

func TestHandoffRouter_WithContext(t *testing.T) {
	// Triage passes additional context to specialist
	triageDecision := HandoffDecision{
		Route:   "technical",
		Context: json.RawMessage(`{"issue_type":"network","priority":"high"}`),
	}
	triageBytes, _ := json.Marshal(triageDecision)

	var receivedData []byte
	client := newMockAgent().
		OnRequest("agent.triage.invoke", triageBytes)
	// Custom handler to capture received data
	client.responses["agent.technical.invoke"] = []byte("fixed")

	router := NewHandoffRouter("support").
		Triage("agent.triage.invoke").
		Route("technical", "agent.technical.invoke")

	_, err := router.Handle(context.Background(), client, []byte("network down"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that technical agent received the context
	for _, call := range client.calls {
		if call.subject == "agent.technical.invoke" {
			receivedData = call.data
		}
	}

	if string(receivedData) != `{"issue_type":"network","priority":"high"}` {
		t.Errorf("expected context to be passed, got '%s'", receivedData)
	}
}

func TestHandoffRouter_UnknownRouteWithFallback(t *testing.T) {
	triageDecision := HandoffDecision{
		Route: "unknown_route",
	}
	triageBytes, _ := json.Marshal(triageDecision)

	client := newMockAgent().
		OnRequest("agent.triage.invoke", triageBytes).
		OnRequest("agent.escalation.invoke", []byte("escalated to human"))

	router := NewHandoffRouter("support").
		Triage("agent.triage.invoke").
		Route("billing", "agent.billing.invoke").
		Fallback("agent.escalation.invoke")

	result, err := router.Handle(context.Background(), client, []byte("weird request"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result) != "escalated to human" {
		t.Errorf("expected escalation response, got '%s'", result)
	}
}

func TestHandoffRouter_UnknownRouteNoFallback(t *testing.T) {
	triageDecision := HandoffDecision{
		Route: "unknown_route",
	}
	triageBytes, _ := json.Marshal(triageDecision)

	client := newMockAgent().
		OnRequest("agent.triage.invoke", triageBytes)

	router := NewHandoffRouter("support").
		Triage("agent.triage.invoke").
		Route("billing", "agent.billing.invoke")

	_, err := router.Handle(context.Background(), client, []byte("weird request"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var handoffErr *HandoffError
	if !errors.As(err, &handoffErr) {
		t.Fatalf("expected HandoffError, got %T", err)
	}

	if handoffErr.Route != "unknown_route" {
		t.Errorf("expected route 'unknown_route', got '%s'", handoffErr.Route)
	}
}

func TestHandoffRouter_TriageError(t *testing.T) {
	expectedErr := errors.New("triage unavailable")
	client := newMockAgent().
		OnRequestError("agent.triage.invoke", expectedErr)

	router := NewHandoffRouter("support").
		Triage("agent.triage.invoke").
		Route("billing", "agent.billing.invoke")

	_, err := router.Handle(context.Background(), client, []byte("request"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var handoffErr *HandoffError
	if !errors.As(err, &handoffErr) {
		t.Fatalf("expected HandoffError, got %T", err)
	}

	if !errors.Is(handoffErr, expectedErr) {
		t.Errorf("expected underlying error %v", expectedErr)
	}
}

func TestHandoffRouter_SpecialistError(t *testing.T) {
	triageDecision := HandoffDecision{Route: "billing"}
	triageBytes, _ := json.Marshal(triageDecision)

	expectedErr := errors.New("billing agent crashed")
	client := newMockAgent().
		OnRequest("agent.triage.invoke", triageBytes).
		OnRequestError("agent.billing.invoke", expectedErr)

	router := NewHandoffRouter("support").
		Triage("agent.triage.invoke").
		Route("billing", "agent.billing.invoke")

	_, err := router.Handle(context.Background(), client, []byte("billing issue"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var handoffErr *HandoffError
	if !errors.As(err, &handoffErr) {
		t.Fatalf("expected HandoffError, got %T", err)
	}

	if handoffErr.Agent != "billing" {
		t.Errorf("expected agent 'billing', got '%s'", handoffErr.Agent)
	}
}

func TestHandoffRouter_ChainedHandoffs(t *testing.T) {
	// Triage -> billing -> account (billing re-routes)
	triageDecision := HandoffDecision{Route: "billing"}
	triageBytes, _ := json.Marshal(triageDecision)

	billingDecision := HandoffDecision{
		Route:   "account",
		Context: json.RawMessage(`{"from":"billing"}`),
	}
	billingBytes, _ := json.Marshal(billingDecision)

	client := newMockAgent().
		OnRequest("agent.triage.invoke", triageBytes).
		OnRequest("agent.billing.invoke", billingBytes).
		OnRequest("agent.account.invoke", []byte("account fixed"))

	router := NewHandoffRouter("support").
		Triage("agent.triage.invoke").
		Route("billing", "agent.billing.invoke").
		Route("account", "agent.account.invoke")

	trace, err := router.HandleWithTrace(context.Background(), client, []byte("issue"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(trace.Output) != "account fixed" {
		t.Errorf("expected 'account fixed', got '%s'", trace.Output)
	}

	// Should have 3 steps: triage, billing, account
	if len(trace.Path) != 3 {
		t.Errorf("expected 3 path steps, got %d", len(trace.Path))
	}
}

func TestHandoffRouter_MaxHandoffsExceeded(t *testing.T) {
	// Create infinite loop scenario
	loopDecision := HandoffDecision{Route: "billing"}
	loopBytes, _ := json.Marshal(loopDecision)

	// Both agents keep routing to each other
	client := newMockAgent().
		OnRequest("agent.triage.invoke", loopBytes).
		OnRequest("agent.billing.invoke", loopBytes)

	router := NewHandoffRouter("support").
		Triage("agent.triage.invoke").
		Route("billing", "agent.billing.invoke").
		MaxHandoffs(3)

	_, err := router.Handle(context.Background(), client, []byte("looping request"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var handoffErr *HandoffError
	if !errors.As(err, &handoffErr) {
		t.Fatalf("expected HandoffError, got %T", err)
	}

	// Should have reached max handoffs
	if len(handoffErr.Path) > 6 { // 3 handoffs * 2 steps each max
		t.Errorf("path too long despite max handoffs: %d steps", len(handoffErr.Path))
	}
}

func TestHandoffRouter_HandleWithTrace(t *testing.T) {
	triageDecision := HandoffDecision{Route: "technical"}
	triageBytes, _ := json.Marshal(triageDecision)

	client := newMockAgent().
		OnRequest("agent.triage.invoke", triageBytes).
		OnRequest("agent.technical.invoke", []byte("resolved"))

	router := NewHandoffRouter("traced-router").
		Triage("agent.triage.invoke").
		Route("technical", "agent.technical.invoke")

	trace, err := router.HandleWithTrace(context.Background(), client, []byte("tech issue"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if trace.Router != "traced-router" {
		t.Errorf("expected router name 'traced-router', got '%s'", trace.Router)
	}

	if len(trace.Path) != 2 {
		t.Fatalf("expected 2 path steps, got %d", len(trace.Path))
	}

	// Check triage step
	if trace.Path[0].Agent != "triage" {
		t.Errorf("step 0: expected agent 'triage', got '%s'", trace.Path[0].Agent)
	}
	if trace.Path[0].Route != "technical" {
		t.Errorf("step 0: expected route 'technical', got '%s'", trace.Path[0].Route)
	}

	// Check technical step
	if trace.Path[1].Agent != "technical" {
		t.Errorf("step 1: expected agent 'technical', got '%s'", trace.Path[1].Agent)
	}
	if string(trace.Path[1].Output) != "resolved" {
		t.Errorf("step 1: expected output 'resolved', got '%s'", trace.Path[1].Output)
	}

	if trace.Duration == 0 {
		t.Error("expected non-zero duration")
	}
}

func TestHandoffRouter_RouteNames(t *testing.T) {
	triageDecision := HandoffDecision{Route: "billing"}
	triageBytes, _ := json.Marshal(triageDecision)

	client := newMockAgent().
		OnRequest("agent.triage.invoke", triageBytes).
		OnRequest("agent.billing.invoke", []byte("done"))

	router := NewHandoffRouter("test").
		Triage("agent.triage.invoke").
		Route("billing", "agent.billing.invoke")

	trace, _ := router.HandleWithTrace(context.Background(), client, []byte("issue"))

	names := trace.RouteNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 route names, got %d", len(names))
	}
	if names[0] != "triage" {
		t.Errorf("expected 'triage', got '%s'", names[0])
	}
	if names[1] != "billing" {
		t.Errorf("expected 'billing', got '%s'", names[1])
	}
}

func TestHandoffError_Error(t *testing.T) {
	// With agent and route
	err1 := &HandoffError{
		Agent: "triage",
		Route: "unknown",
		Err:   errors.New("route not found"),
	}
	expected1 := `handoff failed: agent "triage" tried to route to "unknown": route not found`
	if err1.Error() != expected1 {
		t.Errorf("expected '%s', got '%s'", expected1, err1.Error())
	}

	// With agent only
	err2 := &HandoffError{
		Agent: "billing",
		Err:   errors.New("connection failed"),
	}
	expected2 := `handoff failed: agent "billing": connection failed`
	if err2.Error() != expected2 {
		t.Errorf("expected '%s', got '%s'", expected2, err2.Error())
	}

	// Generic error
	err3 := &HandoffError{
		Err: errors.New("max handoffs exceeded"),
	}
	expected3 := "handoff failed: max handoffs exceeded"
	if err3.Error() != expected3 {
		t.Errorf("expected '%s', got '%s'", expected3, err3.Error())
	}
}

func TestHandoffError_Unwrap(t *testing.T) {
	underlying := errors.New("underlying error")
	err := &HandoffError{Err: underlying}

	if !errors.Is(err, underlying) {
		t.Error("expected Unwrap to return underlying error")
	}
}

func TestHandoffRouter_NonJSONResponse(t *testing.T) {
	// Triage returns non-JSON (plain text) - treated as final response
	client := newMockAgent().
		OnRequest("agent.triage.invoke", []byte("simple answer"))

	router := NewHandoffRouter("support").
		Triage("agent.triage.invoke").
		Route("billing", "agent.billing.invoke")

	result, err := router.Handle(context.Background(), client, []byte("simple question"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result) != "simple answer" {
		t.Errorf("expected 'simple answer', got '%s'", result)
	}
}

func TestHandoffRouter_Reusable(t *testing.T) {
	triageDecision := HandoffDecision{Route: "billing"}
	triageBytes, _ := json.Marshal(triageDecision)

	client := newMockAgent().
		OnRequest("agent.triage.invoke", triageBytes).
		OnRequest("agent.billing.invoke", []byte("handled"))

	router := NewHandoffRouter("reusable").
		Triage("agent.triage.invoke").
		Route("billing", "agent.billing.invoke")

	// Execute twice
	_, err1 := router.Handle(context.Background(), client, []byte("first"))
	_, err2 := router.Handle(context.Background(), client, []byte("second"))

	if err1 != nil || err2 != nil {
		t.Error("router should be reusable")
	}

	// Should have 4 calls (2 requests * 2 agents each)
	if len(client.calls) != 4 {
		t.Errorf("expected 4 calls, got %d", len(client.calls))
	}
}
