# State Backup and Restore

Backup of state to cloud storage is supported. If enabled, every update to state is backed up to a cloud storage bucket. When a new workspace is created, the operator checks if a backup exists. If so, it is restored.

Currently backups to either S3 or GCS buckets are supported.

{{< tabs "backup" >}}
{{< tab "GCS" >}} 

1. Create a GCS bucket:

    ```bash
    gsutil mb gs://my-backup-bucket
    ```

2. Provide the etok operator with the necessary privileges.

    a. Either [create a secret containing a service account key](#credentials), or [setup workload identity](#workload-identity). 

    b. Ensure the service account possesses the following IAM permissions on the bucket:

    ```
    storage.buckets.get
    storage.objects.create
    storage.objects.delete
    storage.objects.get
    ```

3. Install/update the operator, configuring it to use the GCS backup provider, and providing the name of the bucket:

    ```bash
    etok install --backup-provider=gcs --gcs-bucket=backups-bucket
    ```

    If you're using Workload Identity then you'll need to set the service account annotation too:

    ```bash
    etok install --backup-provider=gcs --gcs-bucket=backups-bucket \
        --sa-annotations iam.gke.io/gcp-service-account=[GSA_NAME]@[PROJECT_NAME].iam.gserviceaccount.com
    ```

{{< /tab >}}
{{< tab "S3" >}} 

1. Create an S3 bucket:

    ```bash
    aws s3 mb s3://my-backup-bucket --region eu-west-2
    ```

2. Provide the etok operator with the necessary privileges.

    a. Create a secret containing an AWS access key and secret key.

    b. Ensure the keys belong to a user that can access the bucket. The following IAM policy provides the necessary permissions:

    ```yaml
    {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Action": [
                    "s3:GetObject",
                    "s3:DeleteObject",
                    "s3:PutObject",
                    "s3:AbortMultipartUpload",
                    "s3:ListMultipartUploadParts"
                ],
                "Resource": [
                    "arn:aws:s3:::${BACKUP_BUCKET}/*"
                ]
            },
            {
                "Effect": "Allow",
                "Action": [
                    "s3:ListBucket"
                ],
                "Resource": [
                    "arn:aws:s3:::${BACKUP_BUCKET}"
                ]
            }
        ]
    }
    ```

3. Install/update the operator, configuring it to use the S3 backup provider, providing the name of the bucket and the bucket's region:

    ```bash
    etok install --backup-provider=s3 --s3-bucket=backups-bucket --s3-region=eu-west-2
    ```

{{< /tab >}}
{{< /tabs >}}

To opt a workspace out of automatic backup and restore, pass the `--ephemeral` flag when creating a new workspace with `workspace new`. This is useful if you intend for your workspace to be short-lived.
