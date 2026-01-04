// Package orchestration provides helpers for common agent orchestration patterns.
//
// These patterns simplify building multi-agent systems with Athyr by providing
// reusable, well-tested implementations of common workflows.
//
// # Patterns
//
// Pipeline - Sequential orchestration where agents are chained in linear order.
// Each agent processes the output from the previous agent.
//
//	pipeline := orchestration.NewPipeline("doc-pipeline").
//	    Step("draft", "agent.draft.invoke").
//	    Step("review", "agent.review.invoke").
//	    Step("polish", "agent.polish.invoke")
//
//	result, err := pipeline.Execute(ctx, client, input)
//
// FanOut - Concurrent orchestration where multiple agents run in parallel on the
// same input, with results aggregated using configurable strategies.
//
//	fanout := orchestration.NewFanOut("stock-analysis").
//	    Agent("fundamental", "agent.fundamental.invoke").
//	    Agent("technical", "agent.technical.invoke").
//	    Agent("sentiment", "agent.sentiment.invoke")
//
//	result, err := fanout.Execute(ctx, client, input, AllMustSucceed())
//
// HandoffRouter - Dynamic task routing where a triage agent classifies requests
// and delegates to appropriate specialist agents. Supports chained handoffs,
// fallback agents, and loop detection.
//
//	router := orchestration.NewHandoffRouter("support").
//	    Triage("agent.triage.invoke").
//	    Route("billing", "agent.billing.invoke").
//	    Route("technical", "agent.technical.invoke").
//	    MaxHandoffs(5).
//	    Fallback("agent.escalation.invoke")
//
//	result, err := router.Handle(ctx, client, customerRequest)
//
// GroupChat - Multi-agent collaborative discussion where participants take turns
// contributing to a shared conversation. Supports custom manager strategies and
// consensus detection.
//
//	chat := orchestration.NewGroupChat("product-review").
//	    Manager(RoundRobinManager()).
//	    Participant("pm", "agent.pm.invoke").
//	    Participant("engineer", "agent.engineer.invoke").
//	    MaxRounds(10).
//	    ConsensusCheck(detectAgreement)
//
//	result, err := chat.Discuss(ctx, client, topic)
//
// # Step/Agent Options
//
// Steps and agents can be configured with options:
//
//	Step("slow-step", "agent.slow.invoke",
//	    WithTimeout(30*time.Second),
//	    WithTransform(func(data []byte) ([]byte, error) {
//	        return processData(data), nil
//	    }))
//
//	Agent("slow-agent", "agent.slow.invoke",
//	    WithAgentTimeout(10*time.Second))
//
// # Aggregators
//
// FanOut supports multiple aggregation strategies:
//
//	AllMustSucceed()    // Fails if any agent fails
//	FirstSuccess()      // Returns first successful result
//	CollectAll()        // Collects all results, ignoring failures
//	RequireQuorum(n)    // Requires at least n agents to succeed
//	WithAggregator(fn)  // Custom aggregation function
//
// # Tracing
//
// Use ExecuteWithTrace for debugging and monitoring:
//
//	trace, err := pipeline.ExecuteWithTrace(ctx, client, input)
//	for _, step := range trace.Steps {
//	    fmt.Printf("%s: %v\n", step.Name, step.Duration)
//	}
//
//	trace, err := fanout.ExecuteWithTrace(ctx, client, input)
//	fmt.Printf("Success: %d, Failed: %d\n", trace.SuccessCount(), trace.FailureCount())
package orchestration
