# Atlas

**Status:** BETA - I don't expect breaking changes, but still possible.

Atlas, forced by Zeus to support the heavens and the skies on his shoulders.

[See Full Documentation](docs/index.md)

## Overview

Atlas provides the ability to easily run a secure distributed Thanos deployment. Atlas at it's core is a small set of kubernetes operators that uses services and secrets resources as the underlying source of truth to populate a customized Envoy Aggreggated Service Discovery server which the Envoy proxies connect to and obtain their configurations to create the secure distributed envoy network that Thanos then traverses for connectivity.

Atlas provides Thanos Query the ability to connect to Thanos Sidecars **securely** over HTTP/2 authenticated via Mutual TLS. Additionally when an ingress on the observability cluster (where Atlas is installed) is configured properly, you can access every downstream cluster's individual Prometheus and Alert Manager web interfaces.

Finally Atlas provides the ability for EVERY downstream cluster's Prometheus instances to **securely** send alerts back to the observability alert managers over the HTTP/2 protected by Mutual TLS. This means that you can protect access to the alertmanager with something like an oauth2 proxy and not worry about how to allow the Prometheus instances to authenticate to it for sending alerts.

Atlas does not deploy Thanos or configure Thanos for you. Please see Atlas documentation on how to configure Thanos to use Atlas.

## How It Works

Atlas leverages kubernetes services, label selectors, and annotations to configurable all the necessary components. By default Atlas is configured to work seamlessly with [kube-prometheus-stack]() helm chart.

Atlas uses services defined with external IPs to represent a downstream cluster, when properly annotated it will configure the system for accessing the downstream cluster securely. Once Atlas discovers a properly configured service, it will create a helm-values secret for properly deploying an envoy proxy on the downstream cluster.