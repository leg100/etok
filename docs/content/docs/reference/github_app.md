# Github App

The Github app integrates Github with Etok. The app is a webhook server running on kubernetes, listening for events from Github, invoking Etok runs and reporting information back to Github, including whether the runs were successful as well as log output from terraform. The app is separate to the [operator]({{< ref "docs/reference/operator_install.md" >}}), which must also be installed.

![create-1](/github-app-multiple-workspaces.png)

## Deployment

The app is deployed to the kubernetes cluster using the `github deploy` subcommand:

```bash
etok github deploy --url [WEBHOOK_URL]
```

Note: this command also upgrades an existing deployment.

The command also handles creating the app on Github. This is a necessary step prior to the deployment to the cluster, in order to configure the webhook on your Github account and to assign permissions to the app to access your repos.

It'll create a secret named `creds` containing credentials for authenticating to Github. The presence of the secret determines whether the app has been created or not. So if you need to re-create the app, delete the secret and re-run the `github deploy` subcommand.

The deployment runs in a dedicated namespace, set via the namespace flag `--namespace` flag. The default is `github`.

## Operation

Each commit pushed to a repository triggers a terraform plan. The app looks for workspaces connected to the repository and for each connected workspace it'll trigger a plan.

{{< hint info >}}
For a plan, Etok executes `terraform init` followed by `terraform plan`.
{{< /hint >}}

If you want to re-run a plan, click the `Plan` button.

Should you want to apply the plan, click the `Apply` button, which will run `terraform apply` using the plan file produced from the plan.

{{< hint info >}}
For an apply, Etok executes `terraform init` followed by `terraform apply`.
{{< /hint >}}

## Notation

Each plan is summarised with the following notation:

```text
[NAMESPACE]/[WORKSPACE] #[ITERATION] +[ADDITIONS]/~[UPDATES]/−[DELETIONS]
```

Each re-run of a plan results in a new, incremented, iteration, starting with `#1`.

Once a plan completes, the number of proposed additions, updates and deletions is provided.

An apply uses a similar notation, with its iteration corresponding to its plan.

## Restrictions

The visible log output is restricted to 65535 characters. To view the complete log output run the `kubectl logs` command indicated.
