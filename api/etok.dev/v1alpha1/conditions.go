package v1alpha1

const (
	RestoreFailureCondition = "RestoreFailure"
	BackupFailureCondition  = "BackupFailure"
	CacheFailureCondition   = "CacheFailure"
	PodFailureCondition     = "PodFailure"

	ClientCreateReason      = "ClientCreateFailed"
	BucketNotFoundReason    = "BucketNotFound"
	NothingToRestoreReason  = "NothingToRestore"
	UnexpectedErrorReason   = "UnexpectedError"
	RestoreSuccessfulReason = "RestoreSuccessful"
	BackupSuccessfulReason  = "BackupSuccessful"
	StateNotFoundReason     = "StateNotFound"
	CacheBoundReason        = "CacheBound"
	CacheLostReason         = "CacheLost"
	PodCreatedReason        = "PodCreated"

	// Pending means whatever is being observed is reported to be progressing
	// towards a non-failure state.
	PendingReason = "Pending"
)
