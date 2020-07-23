# Stok

**S**upercharge **T**erraform **O**n **K**ubernetes (or more accurately, a poor man's Terraform Enterprise)

## Requirements

* A kubernetes cluster

## Install

Download and install the CLI from [releases](https://github.com/leg100/stok/releases).

Deploy
[CRDs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) and the operator to your cluster:

```bash
stok generate crds | kubectl create -f -
stok generate operator | kubectl apply -f -
```

## First run

Ensure you're in a directory containing terraform configuration:

```bash
$ cat random.tf
resource "random_id" "test" {
  byte_length = 2
}
```

Create a workspace:

```bash
stok workspace new default
```

Run terraform commands:

```bash
stok init
stok validate
stok plan
stok apply
```

## Usage

Usage is similar to the terraform CLI:

```
Supercharge terraform on kubernetes

Usage:
  stok [command]

Available Commands:
  apply        Run apply
  destroy      Run destroy
  force-unlock Run force-unlock
  generate     Generate stok kubernetes resources
  get          Run get
  help         Help about any command
  import       Run import
  init         Run init
  operator     Run the stok operator
  output       Run output
  plan         Run plan
  refresh      Run refresh
  runner       Run the stok runner
  shell        Run shell
  show         Run show
  taint        Run taint
  untaint      Run untaint
  validate     Run validate
  workspace    Stok workspace management

Flags:
      --debug     Enable debug logging
  -h, --help      help for stok
  -v, --version   version for stok

Use "stok [command] --help" for more information about a command.

```

Commands such as `terraform fmt` or `terraform console` have been left out because there is no purpose to running them on kubernetes.

## RBAC

TODO

## Identity

* [GCP Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity)
* [AWS IAM roles for service accounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html)

## Credentials

Place any credentials inside a kubernetes secret named `stok`. For example, to set credentials for the [AWS provider](https://www.terraform.io/docs/providers/aws/index.html):

```
kubectl create secret generic stok \
  --from-literal=AWS_ACCESS_KEY_ID="youraccesskeyid"  \
  --from-literal=AWS_SECRET_ACCESS_KEY="yoursecretaccesskey"
```

`AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` are then made available as environment variables.

Specific support is provided for the [GCP provider](https://www.terraform.io/docs/providers/google/guides/provider_reference.html#full-reference). The environment variable `GOOGLE_APPLICATION_CREDENTIALS` is set to the file `google-credentials.json`. To populate that file, create a secret like so:

```
kubectl create secret generic stok --from-file=google-credentials.json=[path to service account key]
```
