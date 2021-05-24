## E2E Tests

To run a battery of end-to-end tests against a running kubernetes cluster:

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

### Github App

The e2e test of the github app is disabled by default. The following environment variables need to be set to enable the test.

* `GITHUB_E2E_TEST` - Set to `true` to enable e2e test of github app
* `GITHUB_E2E_REPO_URL` - The URL of a github repo for testing, e.g. `git@github.com:leg100/etok-e2e.git`
* `GITHUB_E2E_REPO_OWNER` - The github owner, e.g. `leg100`
* `GITHUB_E2E_REPO_NAME` - The name of the repo, e.g. `etok-e2e`
* `GITHUB_E2E_URL` - The URL of the deployed webhook server, e.g. `https://webhook.etok.dev`
* `GITHUB_E2E_WEBHOOK_SECRET` - The webhook secret for authenticating to the webhook server, e.g. `da39a3ee5e6b4b0d3255bfef95601890afd80709`
* `GITHUB_E2E_INSTALL_ID` - The github app installation ID, e.g. `10992`
* `GITHUB_TOKEN` - A [personal access token](https://github.com/settings/tokens) with the `repo` scope. Necessary for authenticating to the Github API for various steps of the e2e test, e.g. creating a PR, triggering an apply, etc.

