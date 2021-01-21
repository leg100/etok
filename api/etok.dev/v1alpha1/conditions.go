package v1alpha1

const (
	DoneCondition           = "Done"
	RestoreFailureCondition = "RestoreFailure"
	BackupFailureCondition  = "BackupFailure"
	CacheFailureCondition   = "CacheFailure"
	PodFailureCondition     = "PodFailure"
	RunQueuedCondition      = "Queued"
	RunPodReadyCondition    = "PodReady"
	RunFailedCondition      = "Failed"
	RunCompleteCondition    = "Complete"
	WorkspaceReadyCondition = "Ready"

	ClientCreateReason      = "ClientCreateFailed"
	BucketNotFoundReason    = "BucketNotFound"
	NothingToRestoreReason  = "NothingToRestore"
	UnexpectedErrorReason   = "UnexpectedError"
	RestoreSuccessfulReason = "RestoreSuccessful"
	BackupSuccessfulReason  = "BackupSuccessful"
	StateNotFoundReason     = "StateNotFound"
	CacheBoundReason        = "CacheBound"
	CacheLostReason         = "CacheLost"
	WorkspaceNotFoundReason = "WorkspaceNotFound"
	PodPendingTimeoutReason = "PodPendingTimeout"
	QueueTimeoutReason      = "QueueTimeout"
	PodCreatedReason        = "PodCreated"
	PodPendingReason        = "PodPending"
	PodUnknownReason        = "PodUnknown"
	PodSucceededReason      = "PodSucceeded"
	PodFailedReason         = "PodFailed"
	PodRunningReason        = "PodRunning"
	RunEnqueueTimeoutReason = "EnqueueTimeout"
	RunPodCreatedReason     = "PodCreated"

	RunNotQueueable         = "NotQueuable"
	QueueBacklogReason      = "InBacklog"
	FrontOfQueueReason      = "FrontOfQueue"
	WaitingToBeQueued       = "Waiting"
	RunPendingTimeoutReason = "PodPendingTimeout"

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
