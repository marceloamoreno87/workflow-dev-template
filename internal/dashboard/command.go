// internal/dashboard/command.go
package dashboard

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

var intakeTypes = map[workflow.CommandType]bool{
	workflow.CommandSubmitWork:            true,
	workflow.CommandBeginTriage:           true,
	workflow.CommandAuthorizeWork:         true,
	workflow.CommandRejectWork:            true,
	workflow.CommandBlock:                 true,
	workflow.CommandCancel:                true,
	workflow.CommandBeginSpec:             true,
	workflow.CommandBeginImplementation:   true,
	workflow.CommandApproveSpec:           true,
	workflow.CommandSubmitReview:          true,
	workflow.CommandRequestChanges:        true,
	workflow.CommandApprovePR:             true,
	workflow.CommandBeginDeploy:           true,
	workflow.CommandMarkDeploymentHealthy: true,
	workflow.CommandAcceptFeature:         true,
	workflow.CommandCompleteRollout:       true,
	workflow.CommandFailDeployment:        true,
	workflow.CommandRetryDeployment:       true,
	workflow.CommandResolveBlock:          true,
}

var reasonRequired = map[workflow.CommandType]bool{
	workflow.CommandBlock:          true,
	workflow.CommandCancel:         true,
	workflow.CommandRejectWork:     true,
	workflow.CommandFailDeployment: true,
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	var doc struct {
		AggregateID     string `json:"aggregateId"`
		ExpectedVersion uint64 `json:"expectedVersion"`
		Type            string `json:"type"`
		Reason          string `json:"reason"`
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		http.Error(w, "unreadable body", http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		http.Error(w, "malformed command", http.StatusBadRequest)
		return
	}
	cmdType := workflow.CommandType(doc.Type)
	if !intakeTypes[cmdType] {
		http.Error(w, "unknown command type", http.StatusBadRequest)
		return
	}
	aggregate := strings.TrimSpace(doc.AggregateID)
	if aggregate == "" || utf8.RuneCountInString(aggregate) > 256 {
		http.Error(w, "aggregate required", http.StatusBadRequest)
		return
	}
	if utf8.RuneCountInString(doc.Reason) > 2000 {
		http.Error(w, "reason too long", http.StatusBadRequest)
		return
	}
	if reasonRequired[cmdType] && strings.TrimSpace(doc.Reason) == "" {
		http.Error(w, "reason required", http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, `{"aggregateId":%q,"expectedVersion":%d,"type":%q,"reason":%q,"actorId":"actor/operator","status":"accepted-for-review"}`+"\n",
		aggregate, doc.ExpectedVersion, string(cmdType), doc.Reason)
}
