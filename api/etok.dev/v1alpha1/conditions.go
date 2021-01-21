package v1alpha1

const (
	RunFailedCondition      = "Failed"
	RunCompleteCondition    = "Complete"
	WorkspaceReadyCondition = "Ready"

	PodCreatedReason        = "PodCreated"
	PodPendingReason        = "PodPending"
	PodUnknownReason        = "PodUnknown"
	PodSucceededReason      = "PodSucceeded"
	PodFailedReason         = "PodFailed"
	PodRunningReason        = "PodRunning"
	RunQueuedReason         = "Queued"
	RunUnqueuedReason       = "Unqueued"
	RunEnqueueTimeoutReason = "EnqueueTimeout"
	QueueTimeoutReason      = "QueueTimeout"
	RunPendingTimeoutReason = "PodPendingTimeout"
	WorkspaceNotFoundReason = "WorkspaceNotFound"

	// Pending means whatever is being observed is reported to be progressing
	// towards a non-failure state.
	PendingReason = "Pending"

	// Failure means that an error has occured that is considered unrecoverable,
	// likely rendering the resource unavailable or unable to function.
	FailureReason = "Failure"

	// Deleting means the resource is in the process of being deletion
	DeletionReason = "Deleting"

	// Unknown means state of workspace is unknown (or the state of an essential
	// component is unknown)
	UnknownReason = "Unknown"

	// Ready means the resource and all its components are fully functional
	ReadyReason = "AllSystemsOperational"
)
