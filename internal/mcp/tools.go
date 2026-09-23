// internal/mcp/tools.go
package mcp

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func strArg(args map[string]any, field string) (string, bool) {
	raw, ok := args[field]
	if !ok {
		return "", false
	}
	value, ok := raw.(string)
	return value, ok
}

// scopeWorkItem extracts the workItem argument and binds it to the session item
// before any backend call happens.
func scopeWorkItem(s *Server, args map[string]any) (string, *rpcError) {
	item, ok := strArg(args, "workItem")
	if !ok || item == "" {
		return "", &rpcError{Code: -32602, Message: "workItem required"}
	}
	if item != s.workItem {
		return "", &rpcError{Code: -32602, Message: "work item out of scope"}
	}
	return item, nil
}

func callReadWork(s *Server, args map[string]any) (any, *rpcError) {
	item, rpcErr := scopeWorkItem(s, args)
	if rpcErr != nil {
		return nil, rpcErr
	}
	view, err := s.backend.ReadWork(item)
	if err != nil {
		return nil, &rpcError{Code: -32603, Message: "read failed"}
	}
	return map[string]any{"id": view.ID, "state": view.State, "version": view.Version}, nil
}

func callGetSpec(s *Server, args map[string]any) (any, *rpcError) {
	item, rpcErr := scopeWorkItem(s, args)
	if rpcErr != nil {
		return nil, rpcErr
	}
	doc, err := s.backend.GetSpec(item)
	if err != nil {
		return nil, &rpcError{Code: -32603, Message: "spec unavailable"}
	}
	return map[string]any{"ref": doc.Ref, "body": doc.Body}, nil
}

func callSearchKnowledge(s *Server, args map[string]any) (any, *rpcError) {
	query, ok := strArg(args, "query")
	if !ok {
		return nil, &rpcError{Code: -32602, Message: "query required"}
	}
	query = strings.TrimSpace(query)
	if n := utf8.RuneCountInString(query); n == 0 || n > 500 {
		return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("query length %d", n)}
	}
	limit := 10
	if raw, present := args["limit"]; present {
		number, ok := raw.(float64)
		if !ok {
			return nil, &rpcError{Code: -32602, Message: "limit must be a number"}
		}
		limit = int(number)
		if limit < 1 {
			limit = 1
		}
		if limit > 20 {
			limit = 20
		}
	}
	hits, err := s.backend.SearchKnowledge(query, limit)
	if err != nil {
		return nil, &rpcError{Code: -32603, Message: "search failed"}
	}
	out := make([]map[string]any, 0, len(hits))
	for _, hit := range hits {
		out = append(out, map[string]any{"id": hit.ID, "title": hit.Title, "snippet": hit.Snippet})
	}
	return map[string]any{"hits": out}, nil
}
