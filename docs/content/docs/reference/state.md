# State

Terraform state is stored in a secret using the [kubernetes backend](https://www.terraform.io/docs/backends/types/kubernetes.html). It comes into existence once you run `etok init`. If the workspace is deleted then so is the state.

{{< hint warning >}}
Do not define a backend in your terraform configuration - it will conflict with the configuration Etok automatically installs.
{{< /hint >}}

