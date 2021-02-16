## E2E Tests

Run the following make task to run a battery of end-to-end tests against a running kubernetes cluster:

```bash
make e2e
```

One or more environment variables need to be specified:

* `BACKUP_BUCKET` - GCS bucket to backup state to during the tests

By default the tests assume you're running [kind](https://kind.sigs.k8s.io/). For tests targeting kind you need to also specify:

* `GOOGLE_APPLICATION_CREDENTIALS` - Path to a file containing a service account key with credentials to read and write to `$BACKUP_BUCKET`

To target a GKE cluster, set `ENV=gke` along with:

* `BACKUP_SERVICE_ACCOUNT` - GCP service account with permissions to read and write to `$BACKUP_BUCKET`
* `GKE_IMAGE` - Name to assign to the docker image that is built and pushed, e.g. `eu.gcr.io/my-project/etok`
* `GKE_KUBE_CONTEXT` - Name of the kubectl context for the GKE cluster

Because the GKE tests use workload identity, you need to [set an IAM policy](#workload-identity) on `$BACKUP_SERVICE_ACCOUNT`.
