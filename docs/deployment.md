# Deployment

You should have at least two clusters to take full advantage of Atlas. One to act as the observability cluster and the other as a downstream cluster, if you have more than two clusters, all the others are downstream clusters too.

Atlas should only be installed to the **observability** cluster. All downstream clusters will need an envoy instance deployed, Atlas will provide the necessary helm values to configure the downstream clusters.

!!! important
    It is **HIGHLY** recommend using the same namespace for your observability components, it makes deployment management much easier. The default for atlas is `monitoring`.

## Requirements

- 1 Cluster to act as the Observability Cluster
- 1 Cluster to act as a Downstream Cluster
- Ability to install helm charts
- The envoy helm chart must be installed to an edge node (typically where an ingress instance would be deployed)

### Deploy Prometheus with Thanos Sidecar

It is recommended you use the same namespace like `monitoring` for the deployment of Prometheus and Atlas.

How you deploy Prometheus with the Thanos Sidecar is up to you, however I would recommend simply using the [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack) helm chart as it makes this process very simple and takes care of the more complicated bits for you. If you want Thanos persisting to S3 you can pass your S3 credentials along as well.

!!! note
    When using `kube-prometheus-stack` ensure `servicePerReplica` is enabled for both prometheus and alertmanager sections, this will allow proper routing to each individual instance.

Once you have your Prometheus instances deployed, please make sure to note the service name as it will be necessary for configuring Atlas properly. If you are use `kube-prometheus-stack` most of the defaults will work out of the box. If you are using something non-standard, please make sure that the Prometheus Port and Thanos Sidecar ports are on the service.

## Step 1. Deploying Atlas

The first step is to deploy Atlas to your observability cluster.

```bash
helm install atlas chart/
```

## Step 2. Modify CoreDNS Configuration

!!! note
    I highly recommend using GitOps for modifying and configuring the configmap `kube-system/coredns`

Using your favorite method, you will need to edit the coredns config map in the kube-system namespace.

Add the following to the Corefile section.

```text
    atlas:53 {
        errors
        cache 30
        forward . 10.43.43.10
    }
```

The complete version should look something like the following.

```yaml
apiVersion: v1
data:
  Corefile: |
    .:53 {
        errors
        health
        ready
        kubernetes cluster.local in-addr.arpa ip6.arpa {
          pods insecure
          fallthrough in-addr.arpa ip6.arpa
        }
        hosts /etc/coredns/NodeHosts {
          ttl 60
          reload 15s
          fallthrough
        }
        prometheus :9153
        forward . /etc/resolv.conf
        cache 30
        loop
        reload
        loadbalance
    }
    atlas:53 {
        errors
        cache 30
        forward . 10.43.43.10
    }
  NodeHosts: |
    172.20.0.2 k3d-atlas-server-0
kind: ConfigMap
```

## Step 3. Add Downstream Cluster to Observability Cluster

Telling Atlas about a downstream cluster is as simple as adding a Service resource to your observability cluster or you can use the Atlas binary.

### Using YAML

Be sure to change the `name`, `namespace` and the `externalIPs` section to the appropriate values.

```yaml
apiVersion: v1
kind: Service
metadata:
  labels:
    goatlas.io/cluster: "true"
    goatlas.io/replicas: "1"
  name: downstream1
  namespace: monitoring
spec:
  clusterIP: None
  clusterIPs:
    - None
  externalIPs:
    - 1.1.1.1
  ports:
    - name: prometheus
      port: 9090
      protocol: TCP
      targetPort: 9090
    - name: grpc
      port: 10901
      protocol: TCP
      targetPort: 10901
    - name: http
      port: 10902
      protocol: TCP
      targetPort: 10902
```

### Using the CLI

```bash
atlas cluster-add --name "downstream1" --replicas 1 --external-ip "1.1.1.1" 
```

## Step 4. Deploy Envoy on Downstream Cluster

Atlas generates helm values for the Atlas Envoy Helm Chart for every downstream cluster added. These values come with the necessary seed values to allow initial secure connections to be established. Once comms are established the Envoy Aggreggated Discovery capabilites take over ensuring the downstream envoy instance stays configure properly.

Retrieve the downstream's helm values with the `atlas` or `kubectl`

```bash
atlas cluster-values --name "downstream1" > downstream1.yaml
```

**Note:** This command has `--format` option, the default is `raw` which is just values for helm. The other options are `helm-chart` and `helm-release`

- `helm-chart` -- this is a feature from Rancher on K3S clusters
- `helm-release` -- this is for Flux V2
- `raw` -- just values for helm install/upgrade commands

OR

```bash
kubectl get secret -n monitoring downstream1-envoy-values -o json | jq -r '.data."values.yaml" | base64 -D > downstream1.yaml
```

Once you have the values, install helm on your downstream cluster. Make sure you switch to your downstream cluster context now.

```bash
helm install envoy --values downstream1.yaml chart/
```

## Step 5. Repeat

If you have more than one downstream cluster, repeast steps 3 and 4 until you've added all your clusters.

## Step 6. Configure Downstream Prometheus for Observability Alertmanagers

To take full advantage of what Atlas offers, you can configure your downstream prometheus instances to talk to the alertmanagers in the observability cluster.

You'll need to add an alertmanager entry per the number of alertmanagr instances that are on the observability cluster to the downstream prometheus instance. If you are using the prometheus operator then you can simple add an additional alert managers configuration like the following.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: additional-alertmanager-configs
  namespace: monitoring
data:
  config.yaml: |
    - scheme: http
      static_configs:
      - targets:
        - %s
```
