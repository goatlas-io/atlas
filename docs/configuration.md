
# Configuration

## Ingress Setup

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