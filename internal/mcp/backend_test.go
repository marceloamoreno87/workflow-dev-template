package mcp

import (
	"fmt"
	"strings"
)

// fakeBackend serves canned views and counts calls so tests prove validation and
// scope checks happen before the backend is touched.
type fakeBackend struct {
	reads     int
	specs     int
	searches  int
	lastLimit int
	gates     map[string]bool
	requests  int
	proposals int
}

func (f *fakeBackend) ReadWork(workItem string) (WorkView, error) {
	f.reads++
	return WorkView{ID: workItem, State: "implementing", Version: 3}, nil
}

func (f *fakeBackend) GetSpec(workItem string) (SpecDoc, error) {
	f.specs++
	return SpecDoc{Ref: workItem, Body: "Expose GET /healthz for load-balancer health checks."}, nil
}

func (f *fakeBackend) SearchKnowledge(query string, limit int) ([]KnowledgeHit, error) {
	f.searches++
	f.lastLimit = limit
	if strings.Contains(strings.ToLower(query), "freeze") {
		return []KnowledgeHit{{ID: "deploy-freeze", Title: "Deploy Freeze", Snippet: "Freeze production deploys on Fridays."}}, nil
	}
	return nil, nil
}

func (f *fakeBackend) RunGate(workItem, gate string) (GateResult, error) {
	if f.gates == nil {
		f.gates = map[string]bool{}
	}
	f.gates[gate] = true
	if gate == "smoke" {
		return GateResult{Gate: gate, Output: strings.Repeat("smoke-line\n", 10240)}, nil
	}
	return GateResult{Gate: gate, Output: "ok\n"}, nil
}

func (f *fakeBackend) RequestGate(workItem, kind, reason string) (string, error) {
	f.requests++
	return fmt.Sprintf("mcp-cmd-%d", f.requests), nil
}

func (f *fakeBackend) ProposeKnowledge(conceptID, title, body string, sources []string, reason string) error {
	f.proposals++
	return nil
}
