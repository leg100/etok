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
  apply        Run terraform apply
  destroy      Run terraform destroy
  force-unlock Run terraform force-unlock
  generate     Generate stok kubernetes resources
  get          Run terraform get
  help         Help about any command
  import       Run terraform import
  init         Run terraform init
  operator     Run the stok operator
  output       Run terraform output
  plan         Run terraform plan
  refresh      Run terraform refresh
  runner       Run the stok runner
  sh           Run shell
  show         Run terraform show
  state        Run terraform state
  taint        Run terraform taint
  untaint      Run terraform untaint
  validate     Run terraform validate
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
