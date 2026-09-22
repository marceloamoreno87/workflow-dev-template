package runner

import (
	"strings"
	"testing"
	"time"
)

func validTask() Task {
	return Task{
		Image:         "example/gate@sha256:" + strings.Repeat("a", 64),
		Command:       []string{"/bin/echo", "hi"},
		WorkspaceRoot: "/ws",
		Workdir:       "/ws/.worktrees/demo",
		Env:           map[string]string{"GOFLAGS": "-count=1"},
		Memory:        "512m",
		CPUs:          2,
		Timeout:       5 * time.Minute,
		Network:       "none",
	}
}

func TestValidateTask(t *testing.T) {
	t.Parallel()

	if err := validTask().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRejectBadTasks(t *testing.T) {
	t.Parallel()

	mk := func(mut func(*Task)) Task {
		task := validTask()
		mut(&task)
		return task
	}
	manyEnv := map[string]string{}
	for i := 0; i < 65; i++ {
		manyEnv["K"+string(rune('A'+i%26))+string(rune('0'+i/26))] = "v"
	}
	cases := []struct {
		name string
		task Task
	}{
		{name: "floating tag", task: mk(func(t *Task) { t.Image = "example/gate:latest" })},
		{name: "uppercase image", task: mk(func(t *Task) { t.Image = "Example/gate@sha256:" + strings.Repeat("a", 64) })},
		{name: "wrong digest algo", task: mk(func(t *Task) { t.Image = "example/gate@sha512:" + strings.Repeat("a", 64) })},
		{name: "short digest", task: mk(func(t *Task) { t.Image = "example/gate@sha256:abc" })},
		{name: "empty command", task: mk(func(t *Task) { t.Command = nil })},
		{name: "relative binary", task: mk(func(t *Task) { t.Command = []string{"echo", "hi"} })},
		{name: "too many args", task: mk(func(t *Task) { t.Command = append([]string{"/bin/echo"}, make([]string, 32)...) })},
		{name: "relative root", task: mk(func(t *Task) { t.WorkspaceRoot = "ws" })},
		{name: "workdir outside root", task: mk(func(t *Task) { t.Workdir = "/other/dir" })},
		{name: "workdir is root", task: mk(func(t *Task) { t.Workdir = "/ws" })},
		{name: "bad memory unit", task: mk(func(t *Task) { t.Memory = "512mb" })},
		{name: "memory too small", task: mk(func(t *Task) { t.Memory = "32m" })},
		{name: "memory too large", task: mk(func(t *Task) { t.Memory = "16g" })},
		{name: "zero cpus", task: mk(func(t *Task) { t.CPUs = 0 })},
		{name: "too many cpus", task: mk(func(t *Task) { t.CPUs = 16 })},
		{name: "zero timeout", task: mk(func(t *Task) { t.Timeout = 0 })},
		{name: "timeout too large", task: mk(func(t *Task) { t.Timeout = 3 * time.Hour })},
		{name: "bridged network", task: mk(func(t *Task) { t.Network = "bridge" })},
		{name: "bad env key", task: mk(func(t *Task) { t.Env = map[string]string{"has-dash": "v"} })},
		{name: "too much env", task: mk(func(t *Task) { t.Env = manyEnv })},
		{name: "env value too long", task: mk(func(t *Task) { t.Env = map[string]string{"K": strings.Repeat("v", 4097)} })},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.task.Validate(); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}
