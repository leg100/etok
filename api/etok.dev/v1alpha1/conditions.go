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
)
