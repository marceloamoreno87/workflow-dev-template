// internal/mcp/server.go
package mcp

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var ErrMCP = errors.New("mcp request failed")

const maxLineBytes = 1024 * 1024

const maxArgumentsBytes = 64 * 1024

type Backend interface {
	ReadWork(workItem string) (WorkView, error)
	GetSpec(workItem string) (SpecDoc, error)
	SearchKnowledge(query string, limit int) ([]KnowledgeHit, error)
	RunGate(workItem, gate string) (GateResult, error)
	RequestGate(workItem, kind, reason string) (string, error)
	ProposeKnowledge(conceptID, title, body string, sources []string, reason string) error
}

type WorkView struct {
	ID      string
	State   string
	Version uint64
}

type SpecDoc struct {
	Ref  string
	Body string
}

type KnowledgeHit struct {
	ID      string
	Title   string
	Snippet string
}

type GateResult struct {
	Gate      string
	ExitCode  int
	Output    string
	Truncated bool
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type tool struct {
	name        string
	description string
	schema      map[string]any
	call        func(s *Server, args map[string]any) (any, *rpcError)
}

type Server struct {
	tokenHash [32]byte
	workItem  string
	backend   Backend
	authed    bool
	tools     []tool
}

func stringProp() map[string]any {
	return map[string]any{"type": "string"}
}

func schemaFor(fields ...string) map[string]any {
	props := map[string]any{}
	for _, field := range fields {
		if field == "sources" {
			props[field] = map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
			continue
		}
		props[field] = stringProp()
	}
	return map[string]any{"type": "object", "required": fields, "properties": props}
}

func searchSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"query"},
		"properties": map[string]any{
			"query": stringProp(),
			"limit": map[string]any{"type": "integer"},
		},
	}
}

func notWired(name string) func(s *Server, args map[string]any) (any, *rpcError) {
	return func(s *Server, args map[string]any) (any, *rpcError) {
		return nil, &rpcError{Code: -32603, Message: name + " not implemented"}
	}
}

func NewServer(token, workItem string, backend Backend) *Server {
	s := &Server{tokenHash: sha256.Sum256([]byte(token)), workItem: workItem, backend: backend}
	s.tools = []tool{
		{name: "read_work", description: "Read the bound work item state.", schema: schemaFor("workItem"), call: notWired("read_work")},
		{name: "get_spec", description: "Fetch the spec for the bound work item.", schema: schemaFor("workItem"), call: notWired("get_spec")},
		{name: "search_knowledge", description: "Search project knowledge.", schema: searchSchema(), call: notWired("search_knowledge")},
		{name: "run_gate", description: "Run one declared gate for the bound work item.", schema: schemaFor("workItem", "gate"), call: notWired("run_gate")},
		{name: "request_gate", description: "Request a human gate decision.", schema: schemaFor("workItem", "kind", "reason"), call: notWired("request_gate")},
		{name: "propose_knowledge", description: "Stage a knowledge proposal.", schema: schemaFor("conceptId", "title", "body", "sources", "reason"), call: notWired("propose_knowledge")},
	}
	return s
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

func respond(id any, result any, rpcErr *rpcError) []byte {
	raw, err := json.Marshal(rpcResponse{JSONRPC: "2.0", ID: id, Result: result, Error: rpcErr})
	if err != nil {
		raw, _ = json.Marshal(rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: -32603, Message: "encode failed"}})
	}
	return raw
}

func (s *Server) verifyToken(token string) bool {
	sum := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(sum[:], s.tokenHash[:]) == 1
}

// Handle processes one JSON-RPC message, returning the response bytes or nil for
// notifications. It never panics on malformed input.
func (s *Server) Handle(raw []byte) []byte {
	if len(raw) > maxLineBytes {
		return respond(nil, nil, &rpcError{Code: -32602, Message: "request too large"})
	}
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return respond(nil, nil, &rpcError{Code: -32700, Message: "parse error"})
	}
	if req.Method == "notifications/initialized" {
		return nil
	}
	if req.ID == nil {
		return nil
	}
	switch req.Method {
	case "initialize":
		var params struct {
			AuthToken string `json:"authToken"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return respond(req.ID, nil, &rpcError{Code: -32602, Message: "invalid params"})
		}
		if !s.verifyToken(params.AuthToken) {
			return respond(req.ID, nil, &rpcError{Code: -32001, Message: "auth failed"})
		}
		s.authed = true
		return respond(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]any{"name": "harness-mcp", "version": "1"},
			"boundWorkItem":   s.workItem,
		}, nil)
	case "ping":
		if !s.authed {
			return respond(req.ID, nil, &rpcError{Code: -32001, Message: "not authenticated"})
		}
		return respond(req.ID, map[string]any{}, nil)
	case "tools/list":
		if !s.authed {
			return respond(req.ID, nil, &rpcError{Code: -32001, Message: "not authenticated"})
		}
		descriptors := make([]map[string]any, 0, len(s.tools))
		for _, t := range s.tools {
			descriptors = append(descriptors, map[string]any{
				"name": t.name, "description": t.description, "inputSchema": t.schema,
			})
		}
		return respond(req.ID, map[string]any{"tools": descriptors}, nil)
	case "tools/call":
		if !s.authed {
			return respond(req.ID, nil, &rpcError{Code: -32001, Message: "not authenticated"})
		}
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return respond(req.ID, nil, &rpcError{Code: -32602, Message: "invalid params"})
		}
		if len(req.Params) > maxArgumentsBytes {
			return respond(req.ID, nil, &rpcError{Code: -32602, Message: "arguments too large"})
		}
		for _, t := range s.tools {
			if t.name == params.Name {
				result, rpcErr := t.call(s, params.Arguments)
				return respond(req.ID, result, rpcErr)
			}
		}
		return respond(req.ID, nil, &rpcError{Code: -32602, Message: fmt.Sprintf("unknown tool %q", params.Name)})
	default:
		return respond(req.ID, nil, &rpcError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)})
	}
}

// Serve runs the line-delimited stdio loop until EOF or a fatal read error.
func (s *Server) Serve(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), maxLineBytes)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if res := s.Handle(line); res != nil {
			if _, err := w.Write(append(res, '\n')); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}
