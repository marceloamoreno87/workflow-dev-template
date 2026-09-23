// internal/mcp/mutate.go
package mcp

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const gateOutputCap = 64 * 1024

var declaredGates = map[string]bool{
	"format": true, "lint": true, "typecheck": true,
	"test": true, "build": true, "smoke": true,
}

var requestKinds = map[string]bool{
	"merge": true, "production-deploy": true, "client-acceptance": true,
}

var conceptPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)

func callRunGate(s *Server, args map[string]any) (any, *rpcError) {
	item, rpcErr := scopeWorkItem(s, args)
	if rpcErr != nil {
		return nil, rpcErr
	}
	gate, ok := strArg(args, "gate")
	if !ok || !declaredGates[gate] {
		return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("undeclared gate %q", gate)}
	}
	result, err := s.backend.RunGate(item, gate)
	if err != nil {
		return nil, &rpcError{Code: -32603, Message: "gate failed"}
	}
	output, truncated := result.Output, result.Truncated
	if len(output) > gateOutputCap {
		output = output[:gateOutputCap]
		truncated = true
	}
	return map[string]any{
		"gate": result.Gate, "exitCode": result.ExitCode,
		"output": output, "truncated": truncated,
	}, nil
}

func callRequestGate(s *Server, args map[string]any) (any, *rpcError) {
	item, rpcErr := scopeWorkItem(s, args)
	if rpcErr != nil {
		return nil, rpcErr
	}
	kind, ok := strArg(args, "kind")
	if !ok || !requestKinds[kind] {
		return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("unknown gate kind %q", kind)}
	}
	reason, ok := strArg(args, "reason")
	if !ok {
		return nil, &rpcError{Code: -32602, Message: "reason required"}
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(reason)); n == 0 || n > 2000 {
		return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("reason length %d", n)}
	}
	commandID, err := s.backend.RequestGate(item, kind, strings.TrimSpace(reason))
	if err != nil {
		return nil, &rpcError{Code: -32603, Message: "request failed"}
	}
	return map[string]any{"commandId": commandID, "status": "queued"}, nil
}

func callProposeKnowledge(s *Server, args map[string]any) (any, *rpcError) {
	conceptID, ok := strArg(args, "conceptId")
	if !ok || !conceptPattern.MatchString(conceptID) {
		return nil, &rpcError{Code: -32602, Message: "concept id"}
	}
	title, ok := strArg(args, "title")
	if !ok || strings.TrimSpace(title) == "" || utf8.RuneCountInString(title) > 200 {
		return nil, &rpcError{Code: -32602, Message: "title"}
	}
	body, ok := strArg(args, "body")
	if !ok {
		return nil, &rpcError{Code: -32602, Message: "body required"}
	}
	if n := utf8.RuneCountInString(body); n == 0 || n > 100000 {
		return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("body length %d", n)}
	}
	rawSources, ok := args["sources"]
	if !ok {
		return nil, &rpcError{Code: -32602, Message: "sources required"}
	}
	list, ok := rawSources.([]any)
	if !ok || len(list) == 0 {
		return nil, &rpcError{Code: -32602, Message: "at least one source"}
	}
	sources := make([]string, 0, len(list))
	for _, entry := range list {
		source, ok := entry.(string)
		if !ok || strings.TrimSpace(source) == "" {
			return nil, &rpcError{Code: -32602, Message: "empty source"}
		}
		sources = append(sources, source)
	}
	reason, ok := strArg(args, "reason")
	if !ok || strings.TrimSpace(reason) == "" || utf8.RuneCountInString(reason) > 2000 {
		return nil, &rpcError{Code: -32602, Message: "reason"}
	}
	if err := s.backend.ProposeKnowledge(conceptID, strings.TrimSpace(title), body, sources, strings.TrimSpace(reason)); err != nil {
		return nil, &rpcError{Code: -32603, Message: "proposal failed"}
	}
	return map[string]any{"status": "staged"}, nil
}

// diagBackend is a read-only diagnostic backend for cmd/harness-mcp: reads return
// empty views and mutations report "not wired" so the binary is exercisable end to
// end without pretending to mutate. Daemon wiring replaces it.
type diagBackend struct{}

func DiagBackend() Backend {
	return diagBackend{}
}

func (diagBackend) ReadWork(workItem string) (WorkView, error) {
	return WorkView{ID: workItem}, nil
}

func (diagBackend) GetSpec(workItem string) (SpecDoc, error) {
	return SpecDoc{Ref: workItem}, nil
}

func (diagBackend) SearchKnowledge(query string, limit int) ([]KnowledgeHit, error) {
	return nil, nil
}

func (diagBackend) RunGate(workItem, gate string) (GateResult, error) {
	return GateResult{}, fmt.Errorf("not wired")
}

func (diagBackend) RequestGate(workItem, kind, reason string) (string, error) {
	return "", fmt.Errorf("not wired")
}

func (diagBackend) ProposeKnowledge(conceptID, title, body string, sources []string, reason string) error {
	return fmt.Errorf("not wired")
}
