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

## Queueable Commands

Commands with the ability to alter state are deemed 'queueable'. Only one queueable command at a time can run on a workspace. The currently running command is designated as 'active', and commands waiting to become active wait in a workspace FIFO queue.

All other commands run immediately and concurrently.

## Terraform Flags

Terraform flags need to be passed after a double dash, like so:

```
etok apply -- -auto-approve
```

## RBAC

The `install` command also installs ClusterRoles (and ClusterRoleBindings) for your convenience:

* [etok-user](./config/rbac/user.yaml): includes the permissions necessary for running unprivileged commands
* [etok-admin](./config/rbac/admin.yaml): additional permissions for managing workspaces and running [privileged commands](#privileged-commands)

Amend the bindings accordingly to add/remove users. For example to amend the etok-user binding:

```
kubectl edit clusterrolebinding etok-user
```

Note: To restrict users to individual namespaces you'll want to create RoleBindings referencing the ClusterRoles.

## State

Terraform state is stored in a secret using the [kubernetes backend](https://www.terraform.io/docs/backends/types/kubernetes.html). It comes into existence once you run `etok init`. If the workspace is deleted then so is the state.

### State Persistence

Persistence of state to cloud storage is supported. If enabled, every update to the state is backed up to a cloud storage bucket.

To enable persistence, pass the name of an existing bucket via the `--backup-bucket` flag when creating a new workspace with `workspace new`. If the secret storing the state cannot be found, the workspace checks if a backup exists in the bucket. If found, it restores the state to the secret.

Note: only GCS is supported at present.

The operator is responsible for persisting the state. Therefore be sure to provide the appropriate credentials to the operator at install time. Either provide the path to a file containing a GCP service account key via the `--secret-file` flag, or setup workload identity (see below). The service account needs the following permissions on the bucket:

```
storage.buckets.get
storage.objects.create
storage.objects.delete
storage.objects.get
```

## Credentials

Etok looks for credentials in a secret named `etok`. If found, the credentials contained within are made available to terraform as environment variables.

For instance to set credentials for the [GCP provider](https://www.terraform.io/docs/providers/google/guides/provider_reference.html#full-reference):

```
kubectl create secret generic etok --from-file=GOOGLE_CREDENTIALS=[path to service account key]
```

Or, to set credentials for the [AWS provider](https://www.terraform.io/docs/providers/aws/index.html):

```
kubectl create secret generic etok \
  --from-literal=AWS_ACCESS_KEY_ID="youraccesskeyid"  \
  --from-literal=AWS_SECRET_ACCESS_KEY="yoursecretaccesskey"
```

### Workload Identity

https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity

To use Workload Identity for the operator, first ensure you have a GCP service account (GSA). Then bind a policy to the GSA, like so:

```bash
gcloud iam service-accounts add-iam-policy-binding \
    --role roles/iam.workloadIdentityUser \
    --member "serviceAccount:[PROJECT_ID].svc.id.goog[etok/etok]" \
    [GSA_NAME]@[PROJECT_ID].iam.gserviceaccount.com
```

Where `[etok/etok]` refers to the kubernetes service account (KSA) named `etok` in the namespace `etok` (the installation defaults).

Then install the operator with a service account annotation:

```bash
etok install --sa-annotations iam.gke.io/gcp-service-account=[GSA_NAME]@[PROJECT_ID].iam.gserviceaccount.com
```

To use Workload Identity for workspaces, bind a policy to a GSA, as above, but setting the namespace to that of the workspace. The add the annotation to the KSA named `etok` in the namespace of the workspace:

`kubectl annotate serviceaccounts etok iam.gke.io/gcp-service-account=[GSA_NAME]@[PROJECT_ID].iam.gserviceaccount.com`

(`workspace new` creates the KSA if it doesn't already exist)

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
