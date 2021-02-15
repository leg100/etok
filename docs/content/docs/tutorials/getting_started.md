# Getting Started

This tutorial will guide you through installing and running etok for the first time.

You're expected to be familiar with both terraform and kubernetes.

## Kubernetes Cluster

Firstly ensure you have access to a kubernetes cluster. If you don't have access to a cluster then you might want to install [kind](https://kind.sigs.k8s.io/), a tool for locally running a cluster in a docker container.

Also, in order to install etok, ensure you have extensions permissions on the cluster.

## Install

Download etok from [releases](https://github.com/leg100/etok/releases). Extract the `etok` binary from the zipfile and copy it to a directory in your `PATH`, such as `/usr/local/bin`.

Etok is composed of two components: the **operator** (or server), which runs on the cluster, and the **client** (or CLI), which runs on your workstation.

The operator first needs to be installed. To do so run the following command:


```bash
etok install
```

You should see output related to the the installation:

```text
Creating resource CustomResourceDefinition workspaces.etok.dev
Creating resource CustomResourceDefinition runs.etok.dev
Creating resource ClusterRole etok
Creating resource ClusterRole etok-user
Creating resource ClusterRole etok-admin
Creating resource ClusterRoleBinding etok
Creating resource ClusterRoleBinding etok-user
Creating resource ClusterRoleBinding etok-admin
Creating resource Namespace etok
Creating resource ServiceAccount etok/etok
Creating resource Deployment etok/etok
Waiting for Deployment to be ready
```

It may take up to several minutes to install. The resources are installed into the `etok` namespace.

Any problems can be diagnosed using `kubectl`. To check the status of the deployment run the following command:

```bash
kubectl --namespace=etok get deploy
```

You can also check the versions of the currently deployed components by running the following command:

```bash
> etok version
```

This'll print out both the version and the git commit of the respective components, like so:

```text
Client Version: 0.0.9	adf6e514340e49ba98dc86574990a488b2335965
Server Version: 0.0.9	adf6e514340e49ba98dc86574990a488b2335965
```

(*Server* refers to the operator component)

Once installed you can proceed to creating your first workspace.

## First Workspace

An etok workspace is much the same as a terraform workspace. Unlike terraform, there is no default workspace and a workspace must first be created before you can run terraform commands.

Run the following command to create a workspace named `dev`:

```bash
etok workspace new dev
```

You should see output similar to the following:

```text
Created workspace default/dev
Waiting for workspace pod to be ready...
Requested terraform version is 0.14.3
Current terraform version is 0.14.3
Skipping terraform installation
```

If there are problems you can again help diagnose them using `kubectl`. The check the status of the workspace, run:

```bash
kubectl get ws
```

If all is well then you should see the following:

```text
NAME   PHASE   VERSION   AGE    ACTIVE   QUEUE
dev    ready   0.14.3    113s
```

## First Terraform Run

Now you're ready to run a terraform command. Ensure you're still in the same directory as the one in which you created the workspace.

First, write some terraform configuration. Add the following in a new file named `random.tf`:

```terraform
resource "random_id" "test" {
  byte_length = 2
}
```

Now run the `init` command:

```bash
etok init
```

This command creates a new kubernetes pod, uploads `random.tf` and runs `terraform init` on the pod. And then it'll stream the output to the client:

```text
Initializing the backend...

Successfully configured the backend "kubernetes"! Terraform will automatically
use this backend unless the backend configuration changes.

Initializing provider plugins...
- Finding latest version of hashicorp/random...
- Installing hashicorp/random v3.0.1...
- Installed hashicorp/random v3.0.1 (signed by HashiCorp)

Terraform has created a lock file .terraform.lock.hcl to record the provider
selections it made above. Include this file in your version control repository
so that Terraform can guarantee to make the same selections by default when
you run "terraform init" in the future.

Terraform has been successfully initialized!

You may now begin working with Terraform. Try running "terraform plan" to see
any changes that are required for your infrastructure. All Terraform commands
should now work.

If you ever set or change modules or backend configuration for Terraform,
rerun this command to reinitialize your working directory. If you forget, other
commands will detect it and remind you to do so if necessary.
```

If there are any problems, run `kubectl` to retrieve information about the run:

```bash
kubectl get run
```

It should provide output similar to the following:

```text
NAME        COMMAND   WORKSPACE   PHASE       AGE
run-w6nc4   init      dev         completed   11m
```

`run-w6nc4` is the unique ID of the run. You can use this to probe for more information, for instance to retrieve information about the run's pod:

```bash
kubectl describe pod run-w6nc4
```

You can now proceed to running further terraform commands.

## Terraform Plan and Apply

Having run `init` you can now run a `plan`:

```bash
etok plan
```

This should output:

```text
An execution plan has been generated and is shown below.
Resource actions are indicated with the following symbols:
  + create

Terraform will perform the following actions:

  # random_id.test will be created
  + resource "random_id" "test" {
      + b64_std     = (known after apply)
      + b64_url     = (known after apply)
      + byte_length = 2
      + dec         = (known after apply)
      + hex         = (known after apply)
      + id          = (known after apply)
    }

Plan: 1 to add, 0 to change, 0 to destroy.

------------------------------------------------------------------------

Note: You didn't specify an "-out" parameter to save this plan, so Terraform
can't guarantee that exactly these actions will be performed if
"terraform apply" is subsequently run.
```

Again you can use `kubectl` to diagnose any issues:
```bash
kubectl get run
```

Which should show information about both runs:

```text
NAME        COMMAND   WORKSPACE   PHASE       AGE
run-ir7wb   plan      dev         completed   88s
run-w6nc4   init      dev         completed   21m
```

If the `plan` succeeded you can now run an `apply`:

```bash
etok apply
```

This'll print out the usual terraform output, and prompt for confirmation:

```text
An execution plan has been generated and is shown below.
Resource actions are indicated with the following symbols:
  + create

Terraform will perform the following actions:

  # random_id.test will be created
  + resource "random_id" "test" {
      + b64_std     = (known after apply)
      + b64_url     = (known after apply)
      + byte_length = 2
      + dec         = (known after apply)
      + hex         = (known after apply)
      + id          = (known after apply)
    }

Plan: 1 to add, 0 to change, 0 to destroy.

Do you want to perform these actions?
  Terraform will perform the actions described above.
  Only 'yes' will be accepted to approve.

  Enter a value: yes

random_id.test: Creating...
random_id.test: Creation complete after 0s [id=kOM]

Apply complete! Resources: 1 added, 0 changed, 0 destroyed.
```

## Next Steps

You should now be comfortable with basic usage of etok. However to appreciate its advantages (versus running terraform on your workstation) you'll want to read the documentation further, in particular with regard to RBAC, configuring privileged commands, and the handling of credentials.

