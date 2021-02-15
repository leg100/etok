# Etok

**E**xecute **T**erraform **O**n **K**ubernetes

![demo](./demo.svg)

## Why

* Leverage Kubernetes' RBAC for terraform operations and state
* Single platform for end-user and CI/CD usage
* Queue terraform operations
* Leverage GCP workspace identity and other secret-less mechanisms
* Deploy infrastructure alongside applications

## Requirements

* A kubernetes cluster

## Install

Download and install the CLI from [releases](https://github.com/leg100/etok/releases).

Deploy the kubernetes operator onto your cluster:

```bash
etok install
```

## First run

Create a workspace:

```bash
etok workspace new default
```

Write some terraform configuration:

```bash
$ cat random.tf
resource "random_id" "test" {
  byte_length = 2
}
```

Run terraform commands:

```bash
etok init
etok plan
etok apply
```

## Supported Terraform Commands

* `apply`(Q)
* `console`
* `destroy`(Q)
* `fmt`
* `force-unlock`(Q)
* `get`
* `graph`
* `import`(Q)
* `init`(Q)
* `output`
* `plan`
* `providers`
* `providers lock`
* `refresh`(Q)
* `state list`
* `state mv`(Q)
* `state pull`
* `state push`(Q)
* `state replace-provider`(Q)
* `state rm`(Q)
* `state show`
* `show`
* `taint`(Q)
* `untaint`(Q)
* `validate`

## Additional Commands

* `sh`(Q) - run shell or arbitrary command in workspace

## Privileged Commands

Commands can be specified as privileged. Only users possessing the RBAC permission to update the workspace (see below) can run privileged commands. Specify them via the `--privileged-commands` flag when creating a new workspace with `workspace new`.

## Queueable Commands (Q)

Commands with the ability to alter state are deemed 'queueable': only one queueable command at a time can run on a workspace. The currently running command is designated as 'active', and commands waiting to become active wait in a workspace FIFO queue.

All other commands run immediately and concurrently.

## Terraform Flags

Terraform flags need to be passed after a double dash, like so:

```
etok apply -- -auto-approve
```

## State

Terraform state is stored in a secret using the [kubernetes backend](https://www.terraform.io/docs/backends/types/kubernetes.html). It comes into existence once you run `etok init`. If the workspace is deleted then so is the state.

Note: Do not define a backend in your terraform configuration - it will conflict with the configuration Etok automatically installs.

## Credentials

Etok looks for credentials in a secret named `etok` in the relevant namespace. The credentials contained within are made available as environment variables.

For instance to set credentials for the [Terraform GCP provider](https://www.terraform.io/docs/providers/google/guides/provider_reference.html#full-reference), or for making backups to GCS:

```
kubectl create secret generic etok --from-file=GOOGLE_CREDENTIALS=[path to service account key]
```

Or, to set credentials for the [AWS provider](https://www.terraform.io/docs/providers/aws/index.html), or for making backups to S3:

```
kubectl create secret generic etok \
  --from-literal=AWS_ACCESS_KEY_ID="youraccesskeyid"  \
  --from-literal=AWS_SECRET_ACCESS_KEY="yoursecretaccesskey"
```

## Restrictions

Both the terraform configuration and the terraform state, after compression, are subject to a 1MiB limit. This due to the fact that they are stored in a config map and a secret respectively, and the data stored in either cannot exceed 1MiB.

## FAQ

### What is uploaded to the pod when running a plan/apply/destroy?

The contents of the root module (the current working directory, or the value of the `path` flag) is uploaded. Additionally, if the root module configuration contains references to other modules on the local filesystem, then these too are uploaded, along with all such modules recursively referenced (modules referencing modules, and so forth). The directory structure containing all modules is maintained on the kubernetes pod, ensuring relative references remain valid (e.g. `./modules/vpc` or `../modules/vpc`).

Etok supports the use of a [`.terraformignore`](https://www.terraform.io/docs/backends/types/remote.html#excluding-files-from-upload-with-terraformignore) file. Etok expects to find the file in a directory that is an ancestor of the modules to be uploaded. For example, if the modules to be uploaded are in `/tf/modules/prod` and `/tf/modules/vpc`, then the following paths will be checked:

* `/tf/modules/.terraformignore`
* `/tf/.terraformignore`
* `/.terraformignore`

If not found then the default set of rules apply as documented in the link above.

### How do I optimize performance?

You can reasonably expect commands to start running in less than a couple of seconds. That depends on several factors.

Minimize upload of data. As documented above, use a `.terraformignore` file to skip files you don't need to upload. Pass the flag `-v=3` to see which files are being uploaded and which are ignored.

Disable TTY. Pass the `--no-tty` flag to the command. By default, if a TTY is detected, Etok performs a handshake with the pod which adds a delay. However, disabling TTY means you cannot enter standard input if prompted.

Use fast persistent volume storage class for workspace cache. If you're using GKE, pass `--storage-class=premium-rwo` when creating a new workspace with `workspace new`.

Also, configure the GKE cluster to use the [CSI driver](https://cloud.google.com/kubernetes-engine/docs/how-to/persistent-volumes/gce-pd-csi-driver).

## E2E Tests

```
make e2e
```

You need to specify:

* `BACKUP_BUCKET` - GCS bucket to backup state to during the tests

By default the tests assume you're running [kind](https://kind.sigs.k8s.io/). For tests targeting kind you need to also specify:

* `GOOGLE_APPLICATION_CREDENTIALS` - Path to a file containing a service account key with credentials to read and write to `$BACKUP_BUCKET`

To target a GKE cluster, set `ENV=gke` along with:

* `BACKUP_SERVICE_ACCOUNT` - GCP service account with permissions to read and write to `$BACKUP_BUCKET`
* `GKE_IMAGE` - Name to assign to the docker image that is built and pushed, e.g. `eu.gcr.io/my-project/etok`
* `GKE_KUBE_CONTEXT` - Name of the kubectl context for the GKE cluster

Because the GKE tests use workload identity, you need to [set an IAM policy](#workload-identity) on `$BACKUP_SERVICE_ACCOUNT`.

## Support

Reach out to the [slack channel](https://join.slack.com/t/etokworkspace/shared_invite/zt-lbqgojdj-IS6aDIydMXe2X3EYf8ZRow)
