# Atlas

**Status:** BETA - I don't expect breaking changes, but still possible.

Atlas, forced by Zeus to support the heavens and the skies on his shoulders.

## Overview

Atlas provides the ability to easily run a secure distributed Thanos deployment with little overhead. Atlas is a small set of kubernetes operators that uses services and secrets resources as the underlying source of truth to populate a customized Envoy Aggreggated Service Discovery server which the Envoy proxies connect to and obtain their configurations to create the secure distributed envoy network that Thanos then traverses for connectivity.

Atlas provides Thanos Query the ability to connect to Thanos Sidecars **securely** over HTTP/2 authenticated via Mutual TLS. Additionally when an ingress on the observability cluster (where Atlas is installed) is configured properly, you can access every downstream cluster's individual Prometheus interface and Alert Manager deployed.

Finally Atlas provides the ability for EVERY downstream cluster's Prometheus instances to **securely** send alerts back to the alert managers over the HTTP/2 protected by Mutual TLS. This means that you can protect access to the alertmanager with something like an oauth2 proxy and not worry about how to allow prometheus instances to access it for sending alerts.

### Assumptions

These are assumptions for the initial release with the end goal of supporting enough configurations to allow custom deployments to be used.

- Deployment of Prometheus on Downstream Clusters is done with kube-prometheus-stack and the prometheus operator

### Important

This does not deploy thanos or configure thanos for you. Please see Atlas documentation on how to configure Thanos to use Atlas.

## Tasks

- [ ] Finish documentation for deploying observability cluster envoy proxy
- [ ] Finish documentation for configuring thanos query properly

## Features

- End to End Encryption using HTTP/2 and Mutual TLS
- Automatic Service Discovery for Downstream Cluster Thanos Sidecars
- Access Downstream Prometheus Instances through Envoy Proxy Network
- Access Downstream Alert Manager Instances through Envoy Proxy Network
- Access Downstream Thanos Instances through Envoy Proxy Network
- Allow Downstream Prometheus to send alerts to Observability Alert Manager Instances
- Rotate the TLS material uses by the Envoy Proxies

## How It Works

1. Create Service for Downstream Range with External IPs set, for each prometheus instance add an external IP, even if it's the same IP (which it should)
2. Atlas will create internal services based on the external service and manage them.
3. Atlas will populate a DNS zone file and run a custom CoreDNS instance that the primary CoreDNS will forward requests for to it.
4. Thanos will be configured to do service discovery against a specific hostname provided by the Atlas CoreDNS instance.
5. Atlas provides a seed configuration and PKI for downstream envoy proxy for initial communication, ADS takes over after.
6. Envoy on the observability cluster has direct access to ADS without authentication.

## Deployment

At present, Atlas requires a cluster to act as an observability cluster. Once that is acquired, you will need to install Atlas with helm, followed by obtaining the values for the Atlas Envoy helm install and installing it. Then for every cluster you want to add to Atlas, you will need to first add the cluster, then obtain the helm values for the downstream cluster's envoy helm install.

1. Deploy Atlas with Helm
2. Modify `kube-system/coredns` configmap (ideally with giops) to forward altas TLD to atlas coredns server
3. Generate Downstream Envoy Helm Values
4. Deploy Downstream Envoy Helm Chart
5. Deploy Downstream Prometheus
6. Create Service for Downstream Cluster in Observability Cluster
7. Sit back and enjoy the metrics flowing in!

**Note:** when using `kube-prometheus-stack` ensure `servicePerReplica` is enabled for both prometheus and alertmanager sections.

## Configuration

### Ingress Setup

The helm chart takes care of all ingress for Atlas, however there are additional ingress tweaks you may elect to perform should you want to use the full power of Atlas. On your ingress for `thanos-query` if you modify it to have two path prefixes you can leverage the envoy network that allows thanos to communicate securely to access the individual prometheus, thanos sidecar and alert manager instances that exist in the downstream cluster using the observability envoy proxy.

Essentially what this looks like is that the ingress that manages the flow for the thanos-query, you can add a path prefix for `/prom` and point it to the `envoy` proxy that was deployed on the observability cluster, the result then allows you to hit `/prom/downstream-cluster-name/` in a browser and have direct access to the prometheus instance. This is especially helpful for debugging.

```yaml
    http:
      paths:
      - backend:
          service:
            name: thanos-query
            port:
              number: 9090
        path: /
        pathType: Prefix
      - backend:
          service:
            name: envoy
            port:
              number: 10904
        path: /prom
        pathType: Prefix
```

## Development

1. Create k3s Cluster `k3d cluster create atlas --api-port 192.168.11.103:6556` (subsitute the IP for the docker host IP)
2. Obtain k3s kubeconfig `k3d kubeconfig get atlas > atlas-kubeconfig.yaml`
3. Save the contents from step 2 to a file called `altas-kubeconfig.yaml`
4. Setup your terminal to use the file `export KUBECONFIG=./atlas-kubeconfig.yaml`
5. Now you are setup to do development, when you run atlas, it'll use the correct kubeconfig, alternatively you can specify it on the command line.
