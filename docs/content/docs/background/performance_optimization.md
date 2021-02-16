# How do I optimize performance?

You can reasonably expect commands to start running in less than a couple of seconds. That depends on several factors.

Minimize upload of data. As documented above, use a `.terraformignore` file to skip files you don't need to upload. Pass the flag `-v=3` to see which files are being uploaded and which are ignored.

Disable TTY. Pass the `--no-tty` flag to the command. By default, if a TTY is detected, Etok performs a handshake with the pod which adds a delay. However, disabling TTY means you cannot enter standard input if prompted. Disabling TTY generally shaves off 500-1000ms.

Use fast persistent volume storage class for workspace cache. If you're using GKE, pass `--storage-class=premium-rwo` when creating a new workspace with `workspace new`.

Also, if you're using GKE, configure the cluster to use the [CSI driver](https://cloud.google.com/kubernetes-engine/docs/how-to/persistent-volumes/gce-pd-csi-driver). Anecdotal experience suggests it's faster than the in-tree persistent volume driver.
