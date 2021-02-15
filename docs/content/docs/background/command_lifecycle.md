# Command Lifecycle

A lot goes on under the hood when a command is run. There are many steps that comprise the lifecyle of a command, from the moment the command is entered, through to the streaming of the output, and finally handling the termination of the command.

## Configuration Upload

The terraform configuration on the local disk is uploaded to the cluster. Before this happens, etok needs to check if the configuration references local modules, and follow the references transitively. All the config is then archived into a tarball and a `ConfigMap` resource is created containing the tarball.

## Create Run

In parallel to the upload of the configuration, a `Run` resource is created.
