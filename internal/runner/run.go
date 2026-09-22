// internal/runner/run.go
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

var ErrDocker = errors.New("docker run failed")

const outputCap = 64 * 1024

type Result struct {
	Image     string
	Command   []string
	ExitCode  int
	Stdout    string
	Stderr    string
	Truncated bool
	TimedOut  bool
	Duration  time.Duration
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

func dockerEnv() []string {
	env := []string{"PATH=" + os.Getenv("PATH")}
	for _, key := range []string{"LANG", "LC_ALL", "TZ"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func Run(ctx context.Context, task Task) (Result, error) {
	if err := task.Validate(); err != nil {
		return Result{}, err
	}
	binary, err := exec.LookPath("docker")
	if err != nil {
		return Result{}, fmt.Errorf("%w: docker binary not found", ErrDocker)
	}
	cidDir, err := os.MkdirTemp("", "harness-run-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(cidDir)
	cidfile := cidDir + "/container.cid"
	name := fmt.Sprintf("harness-%d-%d", time.Now().UnixNano(), os.Getpid())
	argv, err := Argv(task, name, cidfile)
	if err != nil {
		return Result{}, err
	}
	runCtx, cancel := context.WithTimeout(ctx, task.Timeout)
	defer cancel()
	start := time.Now()
	cmd := exec.CommandContext(runCtx, binary, argv...)
	cmd.Env = dockerEnv()
	var out, errOut cappedWriter
	out.cap = outputCap
	errOut.cap = outputCap
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	runErr := cmd.Run()
	duration := time.Since(start)
	res := Result{
		Image:     task.Image,
		Command:   append([]string{}, task.Command...),
		Stdout:    out.buf.String(),
		Stderr:    errOut.buf.String(),
		Truncated: out.hit || errOut.hit,
		Duration:  duration,
	}
	if runErr == nil {
		return res, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		res.TimedOut = true
		removeContainer(cidfile)
		return res, fmt.Errorf("%w: timed out after %s", ErrDocker, task.Timeout)
	}
	return res, fmt.Errorf("%w: exit code %d", ErrDocker, res.ExitCode)
}

func removeContainer(cidfile string) {
	raw, err := os.ReadFile(cidfile)
	if err != nil {
		return
	}
	cid := strings.TrimSpace(string(raw))
	if cid == "" {
		return
	}
	binary, err := exec.LookPath("docker")
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, binary, "rm", "-f", cid).Run()
}
