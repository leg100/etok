---
title: Introduction
type: docs
---

# Introduction

Etok is a tool for running terraform commands on a kubernetes cluster. In organizations with teams of engineers working with terraform at scale, you want to ensure terraform operations are performed securely and systematically. Etok leverages kubernetes functionality, such as RBAC, to help teams work with terraform in just such a manner.

## Motivation

Running terraform locally is often a bad idea. You may have to keep credentials for cloud providers on your workstation. You may have to open up firewals to permit connectivity from workstations to cloud APIs. And running `terraform plan` can generate thousands of API calls, in which case the workstation connection can be a bottleneck.

Discrepancies between running terraform locally and running it in CI/CD pipelines are a perpetual source of irritation for engineers. The IAM permissions assigned to the local account versus the CI/CD account are very likely to be different, which means a `terraform plan` that succeeds locally can fail on the CI/CD pipeline, for instance if the CI/CD account is missing a certain permission. Other differences in their respective environments such as firewalls, API connectivity, can cause unexpected errors.

Running commands locally that can manipulate state need careful orchestration. While many state backends support locking, you'd still want to ensure the rest of the team are aware of that you're going to run a command such as `terraform import`, particularly if you intend to run multiple such commands one after the other. You'd rather ensure no CI/CD jobs are going to run and fail when they try to lock the state. And then there's no auditing that you ran `terraform import`.

Running terraform instead on a remote system resolves some of these problems. If you run terraform in the cloud it is provisioning, then securing connectivity to the cloud APIs is easier, and it is closer to the APIs, providing for a faster `terraform plan`.

You can maintain fidelity between end-user and CI/CD usage. The same environment is used in both cases, mitigating the discrepancies and errors mentioned above.

Kubernetes is the ideal platform for this remote system. Kubernetes RBAC provides access control of terraform operations. Its control plane provides the isntrumentation for examing terraform operations.
