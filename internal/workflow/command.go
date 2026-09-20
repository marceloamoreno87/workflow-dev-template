package workflow

const (
	CommandSubmitWork            CommandType = "submit_work"
	CommandBeginTriage           CommandType = "begin_triage"
	CommandAuthorizeWork         CommandType = "authorize_work"
	CommandRejectWork            CommandType = "reject_work"
	CommandBlock                 CommandType = "block"
	CommandCancel                CommandType = "cancel"
	CommandBeginSpec             CommandType = "begin_spec"
	CommandBeginImplementation   CommandType = "begin_implementation"
	CommandApproveSpec           CommandType = "approve_spec"
	CommandSubmitReview          CommandType = "submit_review"
	CommandRequestChanges        CommandType = "request_changes"
	CommandApprovePR             CommandType = "approve_pr"
	CommandBeginDeploy           CommandType = "begin_deploy"
	CommandMarkDeploymentHealthy CommandType = "mark_deployment_healthy"
	CommandAcceptFeature         CommandType = "accept_feature"
	CommandCompleteRollout       CommandType = "complete_rollout"
	CommandFailDeployment        CommandType = "fail_deployment"
	CommandRetryDeployment       CommandType = "retry_deployment"
	CommandResolveBlock          CommandType = "resolve_block"
)

type Command struct {
	ID              CommandID
	AggregateID     WorkItemID
	ExpectedVersion Version
	ActorID         ActorID
	Type            CommandType
	Reason          string
}
