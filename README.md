# Etok

**E**nhanced **T**erraform **O**n **K**ubernetes

## Requirements

* A kubernetes cluster

## Install

Download and install the CLI from [releases](https://github.com/leg100/etok/releases).

Deploy
[CRDs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) and the operator to your cluster:

```bash
etok generate crds | kubectl create -f -
etok generate operator | kubectl apply -f -
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
etok workspace new default/default
```

Run terraform commands:

```bash
etok plan
etok apply
```

## Usage

Usage is similar to the terraform CLI:

```
Usage:
  etok [command]

Available Commands:
  apply       Run terraform apply
  destroy     Run terraform destroy
  generate    Generate deployment resources
  help        Help about any command
  plan        Run terraform plan
  sh          Open shell session
  version     Print client version information
  workspace   etok workspace management

Flags:
      --add_dir_header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files
  -h, --help                             help for etok
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory
      --log_file string                  If non-empty, use this log file
      --log_file_max_size uint           Defines the maximum size a log file can grow to. Unit is megabytes. If the value is 0, the maximum file size is unlimited. (defa
ult 1800)
      --logtostderr                      log to standard error instead of files (default true)
      --skip_headers                     If true, avoid header prefixes in the log messages
      --skip_log_headers                 If true, avoid headers when opening log files
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging

Use "etok [command] --help" for more information about a command.

```

Commands such as `terraform fmt` or `terraform console` have been left out because there is no purpose to running them on kubernetes.

## RBAC

TODO

## Identity

* [GCP Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity)
* [AWS IAM roles for service accounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html)

## Credentials

Credentials placed inside a kubernetes secret named `etok` are made available to terraform as environment variables.

For example, to set credentials for the [AWS provider](https://www.terraform.io/docs/providers/aws/index.html):

```
kubectl create secret generic etok \
  --from-literal=AWS_ACCESS_KEY_ID="youraccesskeyid"  \
  --from-literal=AWS_SECRET_ACCESS_KEY="yoursecretaccesskey"
```

`AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` are then made available as environment variables.

Or, to set credentials for the [GCP provider](https://www.terraform.io/docs/providers/google/guides/provider_reference.html#full-reference):

```
kubectl create secret generic etok --from-file=GOOGLE_CREDENTIALS=[path to service account key]
```

# FAQ

## What is uploaded to the pod when running a plan/apply/destroy?

The contents of the root module (the current working directory, or the value of the `path` flag) is uploaded. Additionally, if the root module configuration contains references to other modules on the local filesystem, then these too are uploaded, along with all such modules recursively referenced (modules referencing modules, and so forth). The directory structure containing all modules is maintained on the kubernetes pod, ensuring relative references remain valid (e.g. `./modules/vpc` or `../modules/vpc`).

Etok supports the use of a [`.terraformignore`](https://www.terraform.io/docs/backends/types/remote.html#excluding-files-from-upload-with-terraformignore) file. Etok expects to find the file in any parent directory that contains the modules to be uploaded. If not found then the default set of rules apply.
