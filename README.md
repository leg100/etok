# stok

**s**upercharged **t**erraform **o**n **k**ubernetes

## install

Create kubernetes secret containing google credentials. These are for the google provider (only the google provider is currently supported). You'll need to have downloaded a key for a service account with sufficient permissions.

```
kubectl create secret generic stok --from-file=google-credentials.json=[path to service account key]
```

Deploy the helm chart to your cluster:

```
helm repo add goalspike https://goalspike-charts.storage.googleapis.com
helm install stok goalspike/stok
```

## usage

Usage is similar to the terraform CLI:

```
Usage:
  stok [command]

Available Commands:
  apply        Run terraform apply
  destroy      Run terraform destroy
  force-unlock Run terraform force-unlock
  get          Run terraform get
  help         Help about any command
  import       Run terraform import
  init         Run terraform init
  output       Run terraform output
  plan         Run terraform plan
  refresh      Run terraform refresh
  show         Run terraform show
  state        Run terraform state
  taint        Run terraform taint
  untaint      Run terraform untaint
  validate     Run terraform validate
  version      Run terraform version

Flags:
      --config string      config file (default is $HOME/.stok.yaml)
  -h, --help               help for stok
      --namespace string   kubernetes namespace (default "default")
      --workspace string   terraform workspace (default "default")

Use "stok [command] --help" for more information about a command.
```

Commands such as `terraform fmt` or `terraform console` have been left out because there is no purpose to running them on kubernetes.
