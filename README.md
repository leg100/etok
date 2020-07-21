# stok

**s**upercharge **t**erraform **o**n **k**ubernetes (or, more accurately, a poor man's Terraform Enterprise)

## install

Download and install the CLI from [releases](https://github.com/leg100/stok/releases).

Deploy the
[CRDs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) to your cluster:

```
stok generate crds | kubectl create -f -
```

Deploy the operator to your cluster:

```
stok generate operator | kubectl apply -f -
```

Create a kubernetes secret containing the credentials you need for terraform.

Google:

```
kubectl create secret generic stok --from-file=google-credentials.json=[path to service account key]
```

AWS:

```
kubectl create secret generic stok --from-literal=AWS_ACCESS_KEY_ID="youraccesskeyid"
--from-literal=AWS_SECRET_ACCESS_KEY="yoursecretaccesskey"
```

## usage

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
