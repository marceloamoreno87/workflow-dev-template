// internal/codex/run.go
package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

var ErrCodex = errors.New("codex thread failed")

var ErrTranscript = errors.New("invalid thread transcript")

var ErrResult = errors.New("invalid thread result")

const transcriptByteCap = 256 * 1024

const transcriptEventCap = 2000

const agentOutputSchema = `{"type":"object","required":["status","summary"],"properties":{"status":{"type":"string","enum":["completed","blocked","failed"]},"summary":{"type":"string"},"changes":{"type":"array","items":{"type":"string"}},"evidenceRefs":{"type":"array","items":{"type":"string"}},"blockers":{"type":"array","items":{"type":"string"}},"usage":{"type":"object","properties":{"inputTokens":{"type":"integer","minimum":0},"outputTokens":{"type":"integer","minimum":0}}},"knowledgeProposals":{"type":"array","items":{"type":"string"}},"privilegedRequests":{"type":"array","items":{"type":"string"}}}}`

type Usage struct {
	InputTokens  int64
	OutputTokens int64
}

type ThreadResult struct {
	Status              string
	Summary             string
	Changes             []string
	EvidenceRefs        []string
	Blockers            []string
	Usage               Usage
	KnowledgeProposals  []string
	PrivilegedRequests  []string
	ThreadID            string
	TranscriptEvents    int
	TranscriptTruncated bool
	TimedOut            bool
	Duration            time.Duration
}

type cappedWriter struct {
	cap int
	buf bytes.Buffer
	hit bool
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	room := w.cap - w.buf.Len()
	if room <= 0 {
		w.hit = true
		return len(p), nil
	}
	if len(p) > room {
		w.buf.Write(p[:room])
		w.hit = true
		return len(p), nil
	}
	w.buf.Write(p)
	return len(p), nil
}

func Run(ctx context.Context, spec ThreadSpec) (ThreadResult, error) {
	if err := spec.Validate(); err != nil {
		return ThreadResult{}, err
	}
	if !spec.Deadline.After(time.Now()) {
		return ThreadResult{}, fmt.Errorf("%w: deadline passed", ErrThread)
	}
	binary, err := exec.LookPath("codex")
	if err != nil {
		return ThreadResult{}, fmt.Errorf("%w: codex binary not found", ErrCodex)
	}
	env, err := Env(spec)
	if err != nil {
		return ThreadResult{}, err
	}
	work, err := os.MkdirTemp("", "harness-thread-")
	if err != nil {
		return ThreadResult{}, err
	}
	defer os.RemoveAll(work)
	schemaFile := work + "/output.schema.json"
	if err := os.WriteFile(schemaFile, []byte(agentOutputSchema), 0o644); err != nil {
		return ThreadResult{}, err
	}
	outFile := work + "/last-message.json"
	argv, err := Argv(spec, schemaFile, outFile)
	if err != nil {
		return ThreadResult{}, err
	}
	runCtx, cancel := context.WithTimeout(ctx, spec.Timeout)
	defer cancel()
	start := time.Now()
	cmd := exec.CommandContext(runCtx, binary, argv...)
	cmd.Env = env
	cmd.Stdin = strings.NewReader(Prompt(spec))
	var stdout, stderr cappedWriter
	stdout.cap = transcriptByteCap
	stderr.cap = 64 * 1024
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	duration := time.Since(start)
	threadID, events, trunc, err := parseTranscript(stdout.buf.Bytes())
	if err != nil {
		return ThreadResult{}, err
	}
	res := ThreadResult{
		ThreadID:            threadID,
		TranscriptEvents:    events,
		TranscriptTruncated: trunc || stdout.hit,
		Duration:            duration,
	}
	if runErr != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			res.TimedOut = true
			return res, fmt.Errorf("%w: timed out", ErrCodex)
		}
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			return res, fmt.Errorf("%w: exit code %d", ErrCodex, exitErr.ExitCode())
		}
		return res, fmt.Errorf("%w: start failed", ErrCodex)
	}
	final, err := parseFinal(outFile)
	if err != nil {
		return ThreadResult{}, err
	}
	final.ThreadID = res.ThreadID
	final.TranscriptEvents = res.TranscriptEvents
	final.TranscriptTruncated = res.TranscriptTruncated
	final.Duration = res.Duration
	return final, nil
}

func parseTranscript(raw []byte) (string, int, bool, error) {
	threadID := ""
	events := 0
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if events >= transcriptEventCap {
			return threadID, events, true, nil
		}
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			return "", 0, false, fmt.Errorf("%w: unparsable event", ErrTranscript)
		}
		events++
		if threadID == "" {
			for _, key := range []string{"thread_id", "threadId"} {
				if value, ok := entry[key].(string); ok && value != "" {
					threadID = value
					break
				}
			}
		}
	}
	return threadID, events, false, nil
}

func parseFinal(path string) (ThreadResult, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ThreadResult{}, fmt.Errorf("%w: unreadable final message", ErrResult)
	}
	var doc struct {
		Status       string   `json:"status"`
		Summary      string   `json:"summary"`
		Changes      []string `json:"changes"`
		EvidenceRefs []string `json:"evidenceRefs"`
		Blockers     []string `json:"blockers"`
		Usage        struct {
			InputTokens  int64 `json:"inputTokens"`
			OutputTokens int64 `json:"outputTokens"`
		} `json:"usage"`
		KnowledgeProposals []string `json:"knowledgeProposals"`
		PrivilegedRequests []string `json:"privilegedRequests"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ThreadResult{}, fmt.Errorf("%w: malformed final message", ErrResult)
	}
	switch doc.Status {
	case "completed", "blocked", "failed":
	default:
		return ThreadResult{}, fmt.Errorf("%w: status", ErrResult)
	}
	if n := len([]rune(doc.Summary)); n == 0 || n > 8000 {
		return ThreadResult{}, fmt.Errorf("%w: summary length %d", ErrResult, n)
	}
	for _, list := range [][]string{doc.Changes, doc.EvidenceRefs, doc.Blockers, doc.KnowledgeProposals, doc.PrivilegedRequests} {
		if len(list) > 64 {
			return ThreadResult{}, fmt.Errorf("%w: list too long", ErrResult)
		}
		for _, item := range list {
			if n := len([]rune(item)); n > 2000 {
				return ThreadResult{}, fmt.Errorf("%w: item too long", ErrResult)
			}
		}
	}
	if doc.Usage.InputTokens < 0 || doc.Usage.OutputTokens < 0 {
		return ThreadResult{}, fmt.Errorf("%w: usage", ErrResult)
	}
	return ThreadResult{
		Status:             doc.Status,
		Summary:            doc.Summary,
		Changes:            doc.Changes,
		EvidenceRefs:       doc.EvidenceRefs,
		Blockers:           doc.Blockers,
		Usage:              Usage{InputTokens: doc.Usage.InputTokens, OutputTokens: doc.Usage.OutputTokens},
		KnowledgeProposals: doc.KnowledgeProposals,
		PrivilegedRequests: doc.PrivilegedRequests,
	}, nil
}
