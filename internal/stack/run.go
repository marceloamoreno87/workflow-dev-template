// internal/stack/run.go
package stack

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

var ErrRun = errors.New("stack gate failed")

const outputCap = 64 * 1024

// Result carries one ordered gate log stream (stdout and stderr merged),
// the exit code, timeout flag, and duration.
type Result struct {
	ExitCode int
	Output   string
	TimedOut bool
	Duration time.Duration
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

func hostEnv() []string {
	env := []string{"PATH=" + os.Getenv("PATH")}
	for _, key := range []string{"LANG", "LC_ALL", "TZ"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func Run(ctx context.Context, argv []string, cwd string, timeout time.Duration) (Result, error) {
	if len(argv) == 0 || argv[0] == "" {
		return Result{}, fmt.Errorf("%w: empty command", ErrRun)
	}
	if cwd == "" {
		return Result{}, fmt.Errorf("%w: working directory required", ErrRun)
	}
	if timeout <= 0 {
		return Result{}, fmt.Errorf("%w: timeout required", ErrRun)
	}
	binary, err := exec.LookPath(argv[0])
	if err != nil {
		return Result{}, fmt.Errorf("%w: binary %q not found", ErrRun, argv[0])
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	cmd := exec.CommandContext(runCtx, binary, argv[1:]...)
	cmd.Dir = cwd
	cmd.Env = hostEnv()
	var out cappedWriter
	out.cap = outputCap
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()
	res := Result{Output: out.buf.String(), Duration: time.Since(start)}
	if runErr == nil {
		return res, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		res.TimedOut = true
		return res, fmt.Errorf("%w: timed out", ErrRun)
	}
	return res, fmt.Errorf("%w: exit code %d", ErrRun, res.ExitCode)
}
