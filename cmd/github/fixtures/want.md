```text
Initializing the backend...

Initializing provider plugins...
- Reusing previous version of hashicorp/null from the dependency lock file
- Using hashicorp/null v3.1.0 from the shared cache directory

Terraform has been successfully initialized!

You may now begin working with Terraform. Try running "terraform plan" to see
any changes that are required for your infrastructure. All Terraform commands
should now work.

If you ever set or change modules or backend configuration for Terraform,
rerun this command to reinitialize your working directory. If you forget, other
commands will detect it and remind you to do so if necessary.

null_resource.null[3]: Refreshing state... [id=5150819032173045538]
null_resource.null[2]: Refreshing state... [id=5771978691832974423]
null_resource.null[1]: Refreshing state... [id=3077276550588621831]
null_resource.null[4]: Refreshing state... [id=5068108415754793123]
null_resource.null[0]: Refreshing state... [id=4255108084159494162]

An execution plan has been generated and is shown below.
Resource actions are indicated with the following symbols:
  + create

Terraform will perform the following actions:

  # null_resource.null[5] will be created
  + resource "null_resource" "null" {
        + id = (known after apply)
    }

  # null_resource.null[6] will be created
  + resource "null_resource" "null" {
        + id = (known after apply)
    }

  # null_resource.null[7] will be created
  + resource "null_resource" "null" {
        + id = (known after apply)
    }

  # null_resource.null[8] will be created
  + resource "null_resource" "null" {
        + id = (known after apply)
    }

  # null_resource.null[9] will be created
  + resource "null_resource" "null" {
        + id = (known after apply)
    }

Plan: 5 to add, 0 to change, 0 to destroy.

------------------------------------------------------------------------

This plan was saved to: run-12345.plan

To perform exactly these actions, run the following command to apply:
    terraform apply "run-12345.plan"
```
