package coolify

import (
	"strings"
	"testing"
)

func validDeploy() DeployRequest {
	return DeployRequest{
		ResourceID:     "resource-1",
		ReleaseTag:     "v1.2.3",
		ReleaseCommit:  strings.Repeat("f", 40),
		FeatureEnabled: false,
	}
}

func TestValidateDeploy(t *testing.T) {
	t.Parallel()

	if err := validDeploy().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRejectBadDeploys(t *testing.T) {
	t.Parallel()

	mk := func(mut func(*DeployRequest)) DeployRequest {
		req := validDeploy()
		mut(&req)
		return req
	}
	for _, tc := range []struct {
		name string
		req  DeployRequest
	}{
		{name: "empty resource", req: mk(func(r *DeployRequest) { r.ResourceID = "" })},
		{name: "bad resource chars", req: mk(func(r *DeployRequest) { r.ResourceID = "res ource!" })},
		{name: "bad tag", req: mk(func(r *DeployRequest) { r.ReleaseTag = "1.2.3" })},
		{name: "bad commit", req: mk(func(r *DeployRequest) { r.ReleaseCommit = "abc" })},
		{name: "feature enabled", req: mk(func(r *DeployRequest) { r.FeatureEnabled = true })},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.req.Validate(); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}

func TestValidateRollback(t *testing.T) {
	t.Parallel()

	req := RollbackRequest{ResourceID: "resource-1", ToDeploymentID: "dep-9", Reason: "health checks red"}
	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}
	empty := req
	empty.Reason = ""
	if err := empty.Validate(); err == nil {
		t.Fatal("expected reason requirement, got none")
	}
}

func TestRejectBadBrokers(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		baseURL string
		token   string
	}{
		{name: "empty url", baseURL: "", token: "tok"},
		{name: "no scheme", baseURL: "broker.local", token: "tok"},
		{name: "userinfo", baseURL: "https://user:pass@broker.local", token: "tok"},
		{name: "empty token", baseURL: "https://broker.local", token: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewBroker(tc.baseURL, tc.token); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}
