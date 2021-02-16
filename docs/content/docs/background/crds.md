---
title: CRDs
---

# CRDs

Etok uses two CRDs (custom resource definitions):

* Run
* Workspace

## Run

Whenever you run a terraform command with etok, a `Run` resource is created:

```bash
> etok plan
```

```bash
> kubectl get run
NAME        COMMAND   WORKSPACE   PHASE       AGE
run-290w7   plan     dev         completed    12s
```

## Workspace

A `Workspace` resource maps to a terraform workspace. The command `etok workspace new` is a convenience method for creating a `Workspace` resource:

```bash
> etok workspace new foo
```

```bash
> kubectl get ws
NAME   PHASE   VERSION   AGE   ACTIVE   QUEUE
foo    ready   0.14.3    22s            
```
