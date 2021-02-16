# Credentials

The operator as well as commands may require credentials. The operator may require credentials for performing [state backups]({{< ref "docs/guides/state_backup.md" >}}) to cloud storage. And commands such as `plan` and `apply` may require credentials for using various terraform providers such as for AWS or GCP. 


{{< hint warning >}}
It's advisable where possible to adopt approaches such as Workload Identity instead. They avoid the need to use credentials, thereby also avoiding the associated overhead and security risks, such as manual rotation, ensuring they are not printed in output, etc.
{{< /hint >}}

Etok looks for credentials in a secret named `etok`. The secret needs to be in the relevant namespace: the operator will look for the secret `etok/etok` (if the default `etok` namespace is used); whereas a command will look for the secret in the namespace of its workspace. For instance if its workspace is in the `dev` namespace,  then the command will look for the secret `dev/etok`.

The credentials are made available to the running process as environment variables. The key is the environment variable name, and the corresponding value is the environment variable value.

For instance to set credentials for the [Terraform GCP provider](https://www.terraform.io/docs/providers/google/guides/provider_reference.html#full-reference), or for making backups to GCS:

```bash
kubectl create secret generic etok --from-file=GOOGLE_CREDENTIALS=[path to service account key]
```

Or, to set credentials for the [AWS provider](https://www.terraform.io/docs/providers/aws/index.html), or for making backups to S3:

```bash
kubectl create secret generic etok \
  --from-literal=AWS_ACCESS_KEY_ID="youraccesskeyid"  \
  --from-literal=AWS_SECRET_ACCESS_KEY="yoursecretaccesskey"
```

