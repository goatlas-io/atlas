# Components

Atlas is made up of 4 components, but only 2 are part of it's source code, the other two are external.

- Controller
- Envoy ADS Server
- CoreDNS
- Envoy Proxy

## Controller

The controller (aka operator) is what ensures all the services, secrets and configmaps are in the correct state.

The controller is responsible for taking the service definitions that represent a downstream cluster and ensure that all subsequent resources are created and managed properly. Atlas creates services for each Thanos replica that exists in the downstream cluster and also generates Helm Values for the Envoy helm chart deployment for the downstream clusters.

There are a number of annotations supported by Atlas that allow for the configuration and tweaking of a downstream cluster should it be necessary, the controller also handles those to ensure that everything is configured properly.

## Envoy ADS Server

The Envoy ADS Server watches the kubernetes cluster for changes to services looking for those with the right annotations to mark them as an Atlas Cluster. It also finds and identifies all the PKI related secrets that have been stored by Atlas as well. The Envoy ADS Server then takes all that data and generates all the various Envoy Proxy configurations and creates snapshots for the Envoy Proxies to obtain. As services and secrets change, the Envoy ADS server automatically re-generates configurations as needed and announces the changes so that the connected Envoy Proxy instances will pick up their new configurations.

## CoreDNS

Atlas creates and keeps up-to-date a DNS zone file based on the service information within the observability cluster, the CoreDNS server deployed by the Atlas Helm Chart is set to read in the zone file and reload it when the file changes.

The Thanos Querier component is the consumer of the DNS zone records. Be defining a DNS service discovery query for Thanos Query to use, it will discover all the downstream cluster Thanos Sidecars and load them.

There are two ways to configure Thanos Query to use the DNS either my modifying the cluster's CoreDNS configuration to point the Atlas TLD to the CoreDNS server deployed by Atlas OR point the Thanos Querier deployment DNS server to the CoreDNS instance deployed by Atlas.

## Envoy Proxy

The Envoy Proxy is used both on the observability cluster and the downstream cluster to provide secure communications between all the Thanos and Prometheus components. By leveraging it's service discovery capabilities, Atlas can generate and maintain all the appropriate configurations and the Envoy Proxy instances simply reconfigure themselves as needed.

The downstream Envoy Proxy instance configurations stay fairly static, since very little changes there, however depending on how often you are adding or removing downstream clusters the observability Envoy Proxy instance's configuration changes quite frequently. However thanks to the Envoy's internal mechanisms these changes happen in real-time and do not require a restart or reload.
