---
title: Introduction
type: docs
---

# Introduction

Etok is a framework for running terraform on a kubernetes cluster. In organizations with teams of engineers working with terraform, it can be difficult managing terraform securely, systematically, and at scale. Etok leverages kubernetes' security, control plane, and scalability to help teams work effectively with terraform.

## Features

* Kubernetes operator
* CLI app `etok` provides for familiar and fast terraform UX
* Granular access control via Kubernetes RBAC
* Single platform suitable for both workstation and CI/CD usage
* Github integration
* Queueable terraform operations
* Built-in state backend with automatic backup to cloud storage
* Credential-free via mechanisms such as GKE Workload Identity
* Plus all the advantages kubernetes has to offer: scaling, security, instrumention, etc

## Motivation

Running terraform locally is often a bad idea. You may have to keep credentials for cloud providers on your workstation. You may have to open up firewalls to permit connectivity from workstations to cloud APIs. And running `terraform plan` can generate thousands of API calls, in which case the workstation connection can be a bottleneck.

Discrepancies between running terraform locally and running it in CI/CD pipelines are a perpetual source of irritation for engineers. The IAM permissions assigned to the local account versus the CI/CD account are very likely to be different, which means a `terraform plan` that succeeds locally can fail on the CI/CD pipeline, for instance if the CI/CD account is missing a certain permission. Other differences in their respective environments such as firewalls, API connectivity, can cause unexpected errors.

Running commands locally that can manipulate state need careful orchestration. While many state backends support locking, you'd still want to ensure the rest of the team are aware that you're going to run a command such as `terraform import`, particularly if you intend to run multiple such commands one after the other. You'd rather ensure no CI/CD jobs are going to run and fail when they try to lock the state. And then there's no auditing that you ran `terraform import`.

Running terraform instead on kubernetes resolves these problems. If you run your cluster in the cloud, then securing connectivity to the cloud APIs is easier, and its proximity to the cloud APIs can only make for a faster `terraform plan`.

The same platform is shared by both users and CI/CD pipelines. Both trigger terraform commands through the use of the `etok` CLI. Sharing the same platform mitigates the unexpected errors on CI/CD pipelines described above.

Kubernetes RBAC provides for granular and tried and tested access control of terraform operations. Its control plane and tooling permits interrogation, monitoring and auditing of terraform operations. And of course its scaling abilities permit massively concurrently use of terraform.

And of course kubernetes opens up all sorts of opportunities for integration, whether with your application deployments or with other kubernetes operators and controllers.
