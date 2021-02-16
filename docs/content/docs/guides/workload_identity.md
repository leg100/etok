---
title: Workload Identity
---

# Workload Identity

[Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity) lets GKE pods assume privileges to Google Cloud without the use of credentials. Etok can use Workload Identity both for terraform and for the operator: terraform can use it to authorize the [Google Cloud](https://registry.terraform.io/providers/hashicorp/google/latest/docs) provider to manage Google Cloud resources; the operator can use it to perform [state backups]({{< ref "docs/guides/state_backup.md" >}}).

## Terminology

With Workload Identity, you configure a Kubernetes service account to act as a Google service account. The following acronyms will be used to clearly differentiate between the two types of service account:

* KSA: Kubernetes Service Account
* GSA: Google Service Account

##  Guide

This guide provides instructions for setting up workload identity both for terraform and for state backups.

{{< tabs "workload_identity" >}}
{{< tab "Terraform" >}} 

1. Ensure your GKE cluster has Workload Identity [enabled](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity#enable_on_cluster).

2. Create a GSA if you haven't got one already:

    ```bash
    gcloud iam service-accounts create GSA_NAME
    ```

3. [Grant](https://cloud.google.com/iam/docs/granting-changing-revoking-access#granting-gcloud-manual) IAM privileges to the GSA. For example, assign the compute engine admin IAM role to permit `terraform apply` to create VMs in a Google Cloud project via the [`google_compute_instance`](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_instance) resource:

    ```bash
    gcloud projects add-iam-policy-binding [PROJECT_ID] \
        --member=serviceAccount:[GSA_EMAIL] --role roles/compute.admin
    ```

4. You now need to determine the KSA to use. Etok configures pods with a KSA named `etok` in the given namespace. For instance, if your workspace is in the namespace `dev`, then any terraform commands you run on that workspace will use the KSA `dev/etok`. When you create a workspace for the first time in a namespace, a KSA named `etok` is automatically created. If you haven't yet created a workspace, then you can manually create the KSA:

    ```bash
    kubectl create serviceaccount etok --namespace [NAMESPACE]
    ```

    To allow the KSA to impersonate the GSA, create an IAM policy between the two:

    ```bash
    gcloud iam service-accounts add-iam-policy-binding \
        --role roles/iam.workloadIdentityUser \
        --member "serviceAccount:[PROJECT_ID].svc.id.goog[NAMESPACE/etok]" \
        [GSA_NAME]@[GSA_PROJECT_ID].iam.gserviceaccount.com
    ```

    Where PROJECT_ID is the project of the GKE cluster, NAMESPACE is the namespace of the KSA named `etok`, and GSA_PROJECT_ID is the project of the GSA.

5. Annotate the KSA with details of the GSA it is impersonating:

    ```bash
    kubectl annotate serviceaccounts --namespace [NAMESPACE] etok \
        iam.gke.io/gcp-service-account=[GSA_NAME]@[PROJECT_ID].iam.gserviceaccount.com
    ```

6. Create a workspace if you haven't already:

    ```bash
    etok workspace new --namespace NAMESPACE WORKSPACE_NAME
    ```

7. Write some terraform configuration to deploy a VM in Google Cloud:

    ```terraform
    resource "google_compute_instance" "default" {
      name         = "test"
      machine_type = "e2-medium"
      zone         = "europe-west2-a"
      project      = "[PROJECT_ID]"

      boot_disk {
        initialize_params {
          image = "debian-cloud/debian-9"
        }
      }

      network_interface {
        network = "default"

        access_config {
          // Ephemeral IP
        }
      }
    }
    ```

8. Run `etok init`:

    ```bash
    etok init
    ```

    ```text
    Initializing the backend...

    Successfully configured the backend "kubernetes"! Terraform will automatically
    use this backend unless the backend configuration changes.

    Initializing provider plugins...
    - Reusing previous version of hashicorp/random from the dependency lock file
    - Finding latest version of hashicorp/google...
    - Installing hashicorp/google v3.56.0...
    - Installed hashicorp/google v3.56.0 (signed by HashiCorp)
    - Installing hashicorp/random v3.0.1...
    - Installed hashicorp/random v3.0.1 (signed by HashiCorp)

    Terraform has made some changes to the provider dependency selections recorded
    in the .terraform.lock.hcl file. Review those changes and commit them to your
    version control system if they represent changes you intended to make.

    Terraform has been successfully initialized!

    You may now begin working with Terraform. Try running "terraform plan" to see
    any changes that are required for your infrastructure. All Terraform commands
    should now work.

    If you ever set or change modules or backend configuration for Terraform,
    rerun this command to reinitialize your working directory. If you forget, other
    commands will detect it and remind you to do so if necessary.
    ```

9. Run `etok apply`:
    ```bash
    etok apply
    ```

    ```text
    An execution plan has been generated and is shown below.
    Resource actions are indicated with the following symbols:
      + create

    Terraform will perform the following actions:

      # google_compute_instance.default will be created
      + resource "google_compute_instance" "default" {
          + can_ip_forward       = false
          + cpu_platform         = (known after apply)
          + current_status       = (known after apply)
          + deletion_protection  = false
          + guest_accelerator    = (known after apply)
          + id                   = (known after apply)
          + instance_id          = (known after apply)
          + label_fingerprint    = (known after apply)
          + machine_type         = "e2-medium"
          + metadata_fingerprint = (known after apply)
          + min_cpu_platform     = (known after apply)
          + name                 = "test"
          + project              = "automatize-admin"
          + self_link            = (known after apply)
          + tags_fingerprint     = (known after apply)
          + zone                 = "europe-west2-a"

          + boot_disk {
              + auto_delete                = true
              + device_name                = (known after apply)
              + disk_encryption_key_sha256 = (known after apply)
              + kms_key_self_link          = (known after apply)
              + mode                       = "READ_WRITE"
              + source                     = (known after apply)

              + initialize_params {
                  + image  = "debian-cloud/debian-9"
                  + labels = (known after apply)
                  + size   = (known after apply)
                  + type   = (known after apply)
                }
            }

          + confidential_instance_config {
              + enable_confidential_compute = (known after apply)
            }

          + network_interface {
              + name               = (known after apply)
              + network            = "default"
              + network_ip         = (known after apply)
              + subnetwork         = (known after apply)
              + subnetwork_project = (known after apply)

              + access_config {
                  + nat_ip       = (known after apply)
                  + network_tier = (known after apply)
                }
            }

          + scheduling {
              + automatic_restart   = (known after apply)
              + on_host_maintenance = (known after apply)
              + preemptible         = (known after apply)

              + node_affinities {
                  + key      = (known after apply)
                  + operator = (known after apply)
                  + values   = (known after apply)
                }
            }
        }

    Plan: 1 to add, 0 to change, 0 to destroy.

    Do you want to perform these actions?
      Terraform will perform the actions described above.
      Only 'yes' will be accepted to approve.

      Enter a value: yes

    google_compute_instance.default: Creating...
    google_compute_instance.default: Still creating... [10s elapsed]
    google_compute_instance.default: Creation complete after 14s [id=projects/automatize-admin/zones/europe-west2-a/instances/test]

    Apply complete! Resources: 1 added, 0 changed, 0 destroyed.
    ```

    This demonstrates terraform has been able to use the privileges conferred by workload identity.
{{< /tab >}} 
{{< tab "Backups" >}} 
1. Ensure your GKE cluster has Workload Identity [enabled](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity#enable_on_cluster).

2. Create a GSA if you haven't got one already:

    ```bash
    gcloud iam service-accounts create GSA_NAME
    ```

3. [Grant](https://cloud.google.com/iam/docs/granting-changing-revoking-access#granting-gcloud-manual) sufficient IAM privileges to the GSA on the GCS bucket you'll use for backups. See the [state backups]({{< ref "docs/guides/state_backup.md" >}}) guide for the exact permissions.

4. Allow the etok operator KSA to impersonate the GSA, create an IAM policy between the two:

    ```bash
    gcloud iam service-accounts add-iam-policy-binding \
        --role roles/iam.workloadIdentityUser \
        --member "serviceAccount:[PROJECT_ID].svc.id.goog[etok/etok]" \
        [GSA_NAME]@[GSA_PROJECT_ID].iam.gserviceaccount.com
    ```

    `[etok/etok]` refers to the KSA named `etok` in the namespace `etok`, which is the default for the operator install. PROJECT_ID is the project of the GKE cluster, and GSA_PROJECT_ID is the project of the GSA.

5. Now follow the [state backups]({{< ref "docs/guides/state_backup.md" >}}) guide to install or upgrade the operator with necessary backup provider configuration as well as the necessary service account annotation to complete the configuration of workoad identity for backups.

    {{< /tab >}} 
    {{< /tabs >}} 
