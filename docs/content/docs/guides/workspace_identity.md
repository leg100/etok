---
title: Workspace Identity
---

# Workspace Identity

https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity

To use Workload Identity for the operator, first ensure you have a GCP service account (GSA). Then bind a policy to the GSA, like so:

```bash
gcloud iam service-accounts add-iam-policy-binding \
    --role roles/iam.workloadIdentityUser \
    --member "serviceAccount:[PROJECT_ID].svc.id.goog[etok/etok]" \
    [GSA_NAME]@[PROJECT_ID].iam.gserviceaccount.com
```

Where `[etok/etok]` refers to the kubernetes service account (KSA) named `etok` in the namespace `etok` (the installation defaults).

Then install the operator with a service account annotation:

```bash
etok install --sa-annotations iam.gke.io/gcp-service-account=[GSA_NAME]@[PROJECT_ID].iam.gserviceaccount.com
```

To use Workload Identity for workspaces, bind a policy to a GSA, as above, but setting the namespace to that of the workspace. The add the annotation to the KSA named `etok` in the namespace of the workspace:

```bash
kubectl annotate serviceaccounts etok iam.gke.io/gcp-service-account=[GSA_NAME]@[PROJECT_ID].iam.gserviceaccount.com
```

(`workspace new` creates the KSA if it doesn't already exist)

