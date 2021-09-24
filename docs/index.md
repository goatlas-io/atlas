# Atlas

Welcome to Atlas, the project that makes running [Thanos](https://thanos.io/) at scale with securely not only possible, but easily. 

!!! note
    First and foremost, thank you to Bartek Plotka who initially shared with me the Envoy strategy a couple years ago that ultimately was my insipiration for this project.

## Overview

This documentation will attempt to describe and explain how this project works and furthermore guide you to running it on your own. We are in alpha/beta quality phase right now, so there are rough edges, please bear with us and feel free to open issues and pull requests.

There are a couple of things you need to understand before getting started. This project is based on the concept of being able to have what we are going to call an `observability cluster`, that is a cluster where atlas will run along side all the thanos components. This project has been tested exclusively with using the [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack) helm chart for deploying prometheus with thanos and alertmanagers. Any other deployment method is theoretically supported, but additional configuration will be required to make it work. All clusters running prometheus with the sidecar are going to be called `downstream clusters` (this even applies to the observability cluster if you are running prometheus there).

For Atlas to work seamlessly with Thanos, it provides a DNS server for DNS discovery for the [thanos query component](https://thanos.io/tip/components/query.md/). You must be able to either modify the default CoreDNS config map for the cluster to add three lines of configuration OR you must be able to override the DNS servers for the thanos query deployment to point to the Atlas DNS server.

Atlas makes use of the [Envoy Proxy](https://www.envoyproxy.io/) and it's [Aggreggated Service Discovery](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/operations/dynamic_configuration) mechanism to dynamically configure the proxies to have all the necessary information for relaying information securely.

### Note about PKI

For the initial release, Atlas manages it's own CA certificate and signing information. It is generated unique per install, this is to reduce friction in getting it deployed. Future versions may support cert-manager.

## Requirements

- Must be able to modify CoreDNS config map for the cluster **OR** override DNS servers for thanos query components.
- Must be able to deploy an Envoy Proxy to each downstream cluster (Atlas provides the Helm Values for the Envoy Chart).
- Must be able to expose ports 10900-10904 on the observability cluster Envoy Proxy (Envoy must handle TLS termination).
- Must be able to expose ports 11901-11904 on the downstream cluster Envy Proxy (Envoy must handle TLS termination).

## Installation

The installation is broken down between the Observability cluster which is where Atlas is installed and then downstream clusters which are what Atlas ends up providing access to. All installations are done via Helm. The Atlas command line binary provides commands to add a cluster and retrieve it's helm values. Once you've retrieved the helm values you can install the envoy proxy on the downstream clusters.
