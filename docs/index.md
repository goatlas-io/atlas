# Atlas

Welcome to Atlas, the project that makes running [Thanos](https://thanos.io/) at scale with securely not only possible, but simple.

!!! note
    First and foremost thank you to Bartek Plotka who initially shared with me the Envoy strategy a couple years ago that ultimately led to the creation of this project.

## Overview

Atlas provides the ability to easily run a secure distributed Thanos deployment. Atlas at it's core is a small set of kubernetes operators that uses services and secrets resources as the underlying source of truth to populate a customized Envoy Aggreggated Service Discovery server which the Envoy proxies connect to and obtain their configurations to create the secure distributed envoy network that Thanos then traverses for connectivity.

Atlas provides Thanos Query the ability to connect to Thanos Sidecars **securely** over HTTP/2 authenticated via Mutual TLS. Additionally when an ingress on the observability cluster (where Atlas is installed) is configured properly, you can access every downstream cluster's individual Prometheus and Alert Manager web interfaces.

Finally Atlas provides the ability for EVERY downstream cluster's Prometheus instances to **securely** send alerts back to the observability alert managers over the HTTP/2 protected by Mutual TLS. This means that you can protect access to the alertmanager with something like an oauth2 proxy and not worry about how to allow the Prometheus instances to authenticate to it for sending alerts.

Atlas does not deploy Thanos or configure Thanos for you. Please see Atlas documentation on how to configure Thanos to use Atlas.

## Features

- End to End Encryption using HTTP/2 and Mutual TLS
- Automatic Service Discovery for Downstream Cluster Thanos Sidecars
- Access Downstream Prometheus Instances through Envoy Proxy Network
- Access Downstream Alert Manager Instances through Envoy Proxy Network
- Access Downstream Thanos Instances through Envoy Proxy Network
- Allow Downstream Prometheus to send alerts to Observability Alert Manager Instances
- Rotate the TLS material uses by the Envoy Proxies

### PKI

For the initial release, Atlas manages it's own CA certificate and signing information. It is generated unique per install, this is to reduce friction in getting it deployed. Future versions may support cert-manager.

## Requirements

- Must be able to modify CoreDNS config map for the cluster **OR** override DNS servers for thanos query components.
- Must be able to deploy an Envoy Proxy to each downstream cluster (Atlas provides the Helm Values for the Envoy Chart).
- Must be able to expose ports 10900-10904 on the observability cluster Envoy Proxy (Envoy must handle TLS termination).
- Must be able to expose ports 11901-11904 on the downstream cluster Envy Proxy (Envoy must handle TLS termination).

## Installation

The installation is broken down between the Observability cluster which is where Atlas is installed and then downstream clusters which are what Atlas ends up providing access to. All deployments are done via Helm. The Atlas command line binary provides commands to add a cluster and retrieve it's helm values. Once you've retrieved the helm values you can install the envoy proxy on the downstream clusters.

See the [full deployment documentation](deployment.md)
