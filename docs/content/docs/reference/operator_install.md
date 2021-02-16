---
weight: 1
---

# Operator Install

The command `etok install` installs the operator component onto the cluster.

It will perform an upgrade if it is already installed. This can also be useful for making configuration changes to an existing installation.

By default, it'll use the namespace `etok`. It'll create the namespace if it doesn't already exist. To use a non-default namespace, pass the `--namespace` flag.

Run the `version` command to retrieve the currently installed operator version:

```bash
> etok version
```

This'll print out both the version and the git commit of both the client and operator components, like so:

```text
Client Version: 0.0.9	adf6e514340e49ba98dc86574990a488b2335965
Server Version: 0.0.9	adf6e514340e49ba98dc86574990a488b2335965
```

(*Server* refers to the operator component)
