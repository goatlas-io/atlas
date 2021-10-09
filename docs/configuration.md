
# Configuration

## Labels

Atlas leverages a few labels to identify which services it should care about, all others it ignores.

- `goatlas.io/cluster` - This is used on a service to identify it as a record with external IPs as a downstream cluster
- `goatlas.io/replicas` - The number of Thanos replicas in the downstream clusters

## Annotations

Atlas currently relies upon existing resource definitions within Kubernetes to work and therefore relies on annotations on services for configuration.

### goatlas.io/envoy-selectors

- **Default:** `app=envoy,release=atlas`
- **Resource:** `service`

This annotation is used to override the selector labels uses to identify the Envoy Proxy service that all traffic should be routed through on the downstream cluster, this is important when you deviate from the default deployment naming conventions.

### goatlas.io/thanos-service

- **Default:** `prometheus-operated.monitoring.svc.cluster.local`
- **Resource:** `service`

This annotation is used to change the default fully qualified domain name on the downstream cluster where the thanos sidecar can be reached.

### goatlas.io/prometheus-service

- **Default:** `prometheus-operated.monitoring.svc.cluster.local`
- **Resource:** `service`

This annotation is used to change the default fully qualified domain name on the downstream cluster where the prometheus instance can be reached.

### goatlas.io/alertmanager-service

- **Default:** `alertmanager-operated.monitoring.svc.cluster.local`
- **Resource:** `service`

This annotation is used to change the default fully qualified domain name on the downstream cluster where the alertmanager instance can be reached.

## Ingress Setup for Prometheus Access

The helm chart takes care of all ingresses for Atlas, however there are additional ingress tweaks you may elect to perform should you want to use the full power of Atlas.

Atlas can provide access to all downstream prometheus instances and alert manager instances using the Envoy Proxy network it establishes, but you have to configure an ingress with path prefixing to route the traffic.

In the real world I use the ingress that I use to expose thanos-query to also expose access to the downstream Prometheus instances and the downstream Alert Managers.

On your ingress modify it to have two path prefixes you can leverage the envoy network that allows thanos to communicate securely to access the individual prometheus, thanos sidecar and alert manager instances that exist in the downstream cluster using the observability envoy proxy.

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
