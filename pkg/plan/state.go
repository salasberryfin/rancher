package plan

// PlanState represents the lifecycle state of a plan execution.
type PlanState string

const (
	PlanStatePending    PlanState = "pending"
	PlanStateInProgress PlanState = "in-progress"
	PlanStateSucceeded  PlanState = "succeeded"
	PlanStateFailed     PlanState = "failed"
	PlanStateCancelled  PlanState = "cancelled"
)

// Secret data field keys written by the agent or orchestrator.
const (
	// SecretPlanStateKey is set by the orchestrator to "pending" and transitioned by the agent.
	SecretPlanStateKey = "plan-state"
	// SecretPlanRevisionKey is a monotonically increasing counter incremented each time a new plan version is loaded.
	SecretPlanRevisionKey = "plan-revision"
	// SecretAppliedOutputKey holds the gzip+base64 encoded output from one-time instructions.
	SecretAppliedOutputKey = "applied-output"
	// SecretAppliedPeriodicOutputKey holds the gzip+base64 encoded output from periodic instructions.
	SecretAppliedPeriodicOutputKey = "applied-periodic-output"
	// SecretProbeStatusesKey holds the current probe health statuses.
	SecretProbeStatusesKey = "probe-statuses"
	// SecretFailureCountKey tracks consecutive failures.
	SecretFailureCountKey = "failure-count"
	// SecretAppliedChecksumKey is used for backward compatibility with orchestrators that do not yet set plan-state.
	SecretAppliedChecksumKey = "applied-checksum"
)

// Annotation and type constants for plan Secrets.
const (
	// SecretTypeMachinePlan is the Kubernetes Secret type for plan Secrets.
	SecretTypeMachinePlan = "rke.cattle.io/machine-plan"
	// AnnotationCancelled when set to "true" on the plan Secret triggers cancellation.
	AnnotationCancelled = "plan.cattle.io/cancelled"
	// AnnotationCancelledValue is the value that triggers cancellation.
	AnnotationCancelledValue = "true"
)
