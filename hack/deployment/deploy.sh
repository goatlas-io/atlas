#!/bin/bash

set -euxo pipefail

cd "$(dirname "$0")" || (echo "Unable to change directory" && exit 1)

OP=${1}

DO_REGION=${DO_REGION:="nyc1"}
DO_SIZE=${DO_SIZE:="s-2vcpu-4gb"}
DO_IMAGE=${DO_IMAGE:="ubuntu-20-04-x64"}
DIGITALOCEAN_SSH_KEYS=${DIGITALOCEAN_SSH_KEYS:-""}

HELM_TAG=${HELM_TAG:="master-5f87f6a"}
HELM_PULLSECRET=${HELM_PULLSECRET:=""}

GITHUB_USERNAME=${GITHUB_USERNAME:-""}
GITHUB_TOKEN=${GITHUB_TOKEN:-""}

NAMESPACE=${NAMESPACE:="monitoring"}

THANOS_VERSION=${THANOS_VERSION:="v0.23.1"}

function setup_userdata {
    cat > userdata.yaml <<EOF
#cloud-config
repo_update: true
repo_upgrade: all

packages:
  - curl

runcmd:
  - "curl -sfL https://get.k3s.io | sh -"

EOF
}

function configure_coredns {
  cat > "observability/coredns.yaml" <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns
  namespace: kube-system

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
EOF
}

function setup_atlas_values {
  local ip_address=$1

  cat > "observability/atlas-values.yaml" <<EOF
image:
  repository: ghcr.io/ekristen/atlas
  tag: $HELM_TAG
  pullPolicy: Always
  pullSecret: $HELM_PULLSECRET

envoyads:
  host: envoy-ads.$ip_address.nip.io

controller:
  envoy:
    host: envoy.$ip_address.nip.io

EOF
}

function setup_am {
    local name=$1

    cat > "$name/am.yaml" <<EOF
---
apiVersion: v1
kind: Service
metadata:
  name: alertmanager0
  namespace: $NAMESPACE
spec:
  ports:
    - name: alertmanager
      port: 11903
      protocol: TCP
      targetPort: alertmanager
  selector:
    app: envoy
    release: atlas-envoy
  type: ClusterIP
status:
  loadBalancer: {}
---
apiVersion: v1
kind: Secret
metadata:
  name: additional-alertmanager-configs
  namespace: $NAMESPACE
stringData:
  config.yaml: |
    - static_configs:
      - targets: ["alertmanager0.$NAMESPACE.svc.cluster.local:11903"]

EOF

}

function setup_thanos {
  local obs_ip_address=$1

  cat > "observability/thanos.yaml" <<EOF
---
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    app.kubernetes.io/component: query-layer
    app.kubernetes.io/instance: thanos-query
    app.kubernetes.io/name: thanos-query
    app.kubernetes.io/version: $THANOS_VERSION
  name: thanos-query
  namespace: $NAMESPACE
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app.kubernetes.io/component: query-layer
    app.kubernetes.io/instance: thanos-query
    app.kubernetes.io/name: thanos-query
    app.kubernetes.io/version: $THANOS_VERSION
  name: thanos-query
  namespace: $NAMESPACE
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/component: query-layer
      app.kubernetes.io/instance: thanos-query
      app.kubernetes.io/name: thanos-query
  template:
    metadata:
      labels:
        app.kubernetes.io/component: query-layer
        app.kubernetes.io/instance: thanos-query
        app.kubernetes.io/name: thanos-query
        app.kubernetes.io/version: $THANOS_VERSION
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - podAffinityTerm:
                labelSelector:
                  matchExpressions:
                    - key: app.kubernetes.io/name
                      operator: In
                      values:
                        - thanos-query
                namespaces:
                  - thanos
                topologyKey: kubernetes.io/hostname
              weight: 100
      containers:
        - args:
            - query
            - --grpc-address=0.0.0.0:10901
            - --http-address=0.0.0.0:9090
            - --log.level=info
            - --log.format=logfmt
            - --query.replica-label=prometheus_replica
            - --query.replica-label=rule_replica
            - --query.auto-downsampling
            - --store=dnssrvnoa+_grpc._tcp.prometheus-operated.monitoring.svc.cluster.local.
            - --store=dnssrvnoa+_thanos._tcp.sidecars.thanos.atlas.
          env:
            - name: HOST_IP_ADDRESS
              valueFrom:
                fieldRef:
                  fieldPath: status.hostIP
          image: quay.io/thanos/thanos:$THANOS_VERSION
          livenessProbe:
            failureThreshold: 4
            httpGet:
              path: /-/healthy
              port: 9090
              scheme: HTTP
            periodSeconds: 30
          name: thanos-query
          ports:
            - containerPort: 10901
              name: grpc
            - containerPort: 9090
              name: http
          readinessProbe:
            failureThreshold: 20
            httpGet:
              path: /-/ready
              port: 9090
              scheme: HTTP
            periodSeconds: 5
          resources: {}
          terminationMessagePolicy: FallbackToLogsOnError
      securityContext:
        fsGroup: 65534
        runAsUser: 65534
      serviceAccountName: thanos-query
      terminationGracePeriodSeconds: 120
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/component: query-layer
    app.kubernetes.io/instance: thanos-query
    app.kubernetes.io/name: thanos-query
    app.kubernetes.io/version: $THANOS_VERSION
  name: thanos-query
  namespace: $NAMESPACE
spec:
  ports:
    - name: grpc
      port: 10901
      targetPort: 10901
    - name: http
      port: 9090
      targetPort: 9090
  selector:
    app.kubernetes.io/component: query-layer
    app.kubernetes.io/instance: thanos-query
    app.kubernetes.io/name: thanos-query
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: thanos-query
  namespace: $NAMESPACE
spec:
  rules:
    - host: thanos-query.$obs_ip_address.nip.io
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
          - backend:
              service:
                name: kube-prometheus-stack-prometheus
                port:
                  number: 9090
            path: /prom-local
            pathType: Prefix

EOF

}

function setup_downstream_prometheus {
  local name=$1
  local ip_address=$2
  local obs_ip_address=$3

  cat > "$name/kube-prometheus-stack-values.yaml" << EOF
grafana:
  enabled: false
alertmanager:
  enabled: true
  ingress:
    enabled: true
    hosts:
      - alerts.$ip_address.nip.io
    pathType: Prefix
  servicePerReplica:
    enabled: true
  alertmanagerSpec:
    replicas: 1
    externalUrl: http://alerts.$obs_ip_address.nip.io
prometheus:
  enabled: true
  ingress:
    enabled: true
    hosts:
      - prometheus.$ip_address.nip.io
    paths:
      - /
    pathType: Prefix
  ingressPerReplica:
    enabled: true
    hostPrefix: prometheus
    hostDomain: $ip_address.nip.io
    paths:
      - /
    pathType: Prefix
  thanosIngress:
    enabled: true
    hosts:
      - thanos-sidecar.$ip_address.nip.io
    paths:
      - /
    pathType: Prefix
  thanosService:
    enabled: true
  servicePerReplica:
    enabled: true
  prometheusSpec:
    podMonitorNamespaceSelector: {}
    podMonitorSelector: {}
    serviceMonitorSelector: {}
    serviceMonitorNamespaceSelector: {}
    additionalScrapeConfigsSecret:
      enabled: false
      key: config.yaml
      name: prometheus-scrape-configs
    additionalAlertManagerConfigsSecret:
      key: config.yaml
      name: additional-alertmanager-configs
    replicaExternalLabelName: prometheus_replica
    prometheusExternalLabelName: prometheus_group
    replicas: 1
    thanos:
      image: docker.io/thanosio/thanos:$THANOS_VERSION
    externalLabels:
      prometheus_group: "$name"
      prometheus_replica: "\$(HOSTNAME)"

EOF
}

function setup_observability_prometheus {
  local ip_address=$1

  cat > "observability/kube-prometheus-stack-values.yaml" << EOF
grafana:
  enabled: false
alertmanager:
  enabled: true
  ingress:
    enabled: true
    hosts:
      - alerts.$ip_address.nip.io
    paths:
      - /
    pathType: Prefix
  servicePerReplica:
    enabled: true
  alertmanagerSpec:
    replicas: 1
    externalUrl: http://alerts.$ip_address.nip.io
prometheus:
  enabled: true
  ingress:
    enabled: true
    hosts:
      - prometheus.$ip_address.nip.io
    paths:
      - /
    pathType: Prefix
  ingressPerReplica:
    enabled: true
    hostPrefix: prometheus
    hostDomain: $ip_address.nip.io
    paths:
      - /
    pathType: Prefix
  thanosIngress:
    enabled: true
    hosts:
      - thanos-sidecar.$ip_address.nip.io
    paths:
      - /
    pathType: Prefix
  thanosService:
    enabled: true
  servicePerReplica:
    enabled: true
  prometheusSpec:
    podMonitorNamespaceSelector: {}
    podMonitorSelector: {}
    serviceMonitorSelector: {}
    serviceMonitorNamespaceSelector: {}
    additionalScrapeConfigsSecret:
      enabled: false
      key: config.yaml
      name: prometheus-scrape-configs
    externalLabels:
      prometheus_group: "observability"
      prometheus_replica: "\$(HOSTNAME)"
    replicaExternalLabelName: prometheus_replica
    prometheusExternalLabelName: prometheus_group
    replicas: 1
    thanos:
      image: docker.io/thanosio/thanos:$THANOS_VERSION
EOF
}

function setup_observability {
    mkdir -p observability

    if ! doctl compute droplet get observability > /dev/null 2>&1; then
        doctl compute droplet create observability --ssh-keys "$DIGITALOCEAN_SSH_KEYS" --user-data-file userdata.yaml --region "$DO_REGION" --size "$DO_SIZE" --image "$DO_IMAGE" -o json --wait > observability/droplet.json

        IP_ADDRESS=$(jq -r '.[0].networks.v4[0].ip_address' < observability/droplet.json)

        while ! ssh -o "StrictHostKeyChecking no" "root@$IP_ADDRESS" ls /etc/rancher/k3s/k3s.yaml; do
            echo "Waiting for k3s cluster to bootstrap"
            sleep 2
        done
    fi

    if [ ! -f observability/kubeconfig.yaml ]; then
        IP_ADDRESS=$(jq -r '.[0].networks.v4[0].ip_address' < observability/droplet.json)

        scp -o "StrictHostKeyChecking no" "root@$IP_ADDRESS:/etc/rancher/k3s/k3s.yaml" observability/kubeconfig.yaml

        sed -i.bak "s/127.0.0.1/$IP_ADDRESS/g" observability/kubeconfig.yaml
        rm -f observability/kubeconfig.yaml.bak
    fi

    if ! KUBECONFIG=observability/kubeconfig.yaml kubectl get namespace $NAMESPACE; then
        KUBECONFIG=observability/kubeconfig.yaml kubectl create ns $NAMESPACE
    fi

    if [ -n "$GITHUB_USERNAME" ] && [ -n "$GITHUB_TOKEN" ]; then
      if ! KUBECONFIG=observability/kubeconfig.yaml kubectl get secret github -n $NAMESPACE; then
          KUBECONFIG=observability/kubeconfig.yaml kubectl create secret docker-registry github --docker-server=https://ghcr.io --docker-username="$GITHUB_USERNAME" --docker-password="$GITHUB_TOKEN" -n "$NAMESPACE"
      fi
    fi

    IP_ADDRESS=$(jq -r '.[0].networks.v4[0].ip_address' < observability/droplet.json)

    setup_observability_prometheus "$IP_ADDRESS"

    helm upgrade --kubeconfig "observability/kubeconfig.yaml" -i -n $NAMESPACE --values "observability/kube-prometheus-stack-values.yaml" prometheus prometheus-community/kube-prometheus-stack
    
    configure_coredns
    setup_thanos "$IP_ADDRESS"

    KUBECONFIG=observability/kubeconfig.yaml kubectl apply -f observability/thanos.yaml
    KUBECONFIG=observability/kubeconfig.yaml kubectl apply -f observability/coredns.yaml

    setup_atlas_values "$IP_ADDRESS"

    helm upgrade --kubeconfig observability/kubeconfig.yaml -i -n $NAMESPACE --values observability/atlas-values.yaml atlas ../../charts/atlas
}

function setup_downstream {
    local name=$1

    mkdir -p "$name"

    if ! doctl compute droplet get "$name" 2>/dev/null; then
        doctl compute droplet create "$name" --ssh-keys "$DIGITALOCEAN_SSH_KEYS" --user-data-file userdata.yaml --region "$DO_REGION" --size "$DO_SIZE" --image "$DO_IMAGE" -o json --wait > "$name/droplet.json"

        IP_ADDRESS=$(jq -r '.[0].networks.v4[0].ip_address' < "$name/droplet.json")

        while ! ssh -o "StrictHostKeyChecking no" "root@$IP_ADDRESS" ls /etc/rancher/k3s/k3s.yaml; do
            echo "Waiting for k3s cluster to bootstrap"
            sleep 2
        done
    fi

    if [ ! -f "$name/kubeconfig.yaml" ]; then
        IP_ADDRESS=$(jq -r '.[0].networks.v4[0].ip_address' < "$name/droplet.json")

        scp -o "StrictHostKeyChecking no" "root@$IP_ADDRESS:/etc/rancher/k3s/k3s.yaml" "$name/kubeconfig.yaml"

        sed -i.bak "s/127.0.0.1/$IP_ADDRESS/g" "$name/kubeconfig.yaml"
        rm -f "$name/kubeconfig.yaml.bak"
    fi

    if ! KUBECONFIG="$name/kubeconfig.yaml" kubectl get namespace $NAMESPACE; then
        KUBECONFIG="$name/kubeconfig.yaml" kubectl create ns $NAMESPACE
    fi

    if [ -n "$GITHUB_USERNAME" ] && [ -n "$GITHUB_TOKEN" ]; then
      if ! KUBECONFIG=observability/kubeconfig.yaml kubectl get secret github -n $NAMESPACE; then
          KUBECONFIG=observability/kubeconfig.yaml kubectl create secret docker-registry github --docker-server=https://ghcr.io --docker-username="$GITHUB_USERNAME" --docker-password="$GITHUB_TOKEN" -n "$NAMESPACE"
      fi
    fi

    IP_ADDRESS=$(jq -r '.[0].networks.v4[0].ip_address' < "$name/droplet.json")
    
    setup_downstream_prometheus "$name" "$IP_ADDRESS" "$(jq -r '.[0].networks.v4[0].ip_address' < observability/droplet.json)"

    helm upgrade --kubeconfig "$name/kubeconfig.yaml" -i -n $NAMESPACE --values "$name/kube-prometheus-stack-values.yaml" prometheus prometheus-community/kube-prometheus-stack
}

function config_atlas_for_cluster() {
    local name=$1

    if [ ! -d "$name" ]; then
        echo "ERROR: invalid cluster name"
        return 1
    fi

    IP_ADDRESS=$(jq -r '.[0].networks.v4[0].ip_address' < "$name/droplet.json")

    KUBECONFIG="observability/kubeconfig.yaml" kubectl apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  labels:
    goatlas.io/cluster: "true"
    goatlas.io/replicas: "1"
  name: $name
  namespace: $NAMESPACE
spec:
  clusterIP: None
  clusterIPs:
    - None
  externalIPs:
    - $IP_ADDRESS
  ports:
    - name: prometheus
      port: 9090
      protocol: TCP
      targetPort: 9090
    - name: thanos
      port: 10901
      protocol: TCP
      targetPort: 10901
    - name: alertmanager
      port: 9093
      protocol: TCP
      targetPort: 9093
EOF

    while ! KUBECONFIG="observability/kubeconfig.yaml" kubectl get secret "$name-envoy-values" -n $NAMESPACE; do
        echo "Waiting for envoy-values secret to become available"
        sleep 2
    done
}   

function config_cluster_envoy() {
    local name=$1

    if [ ! -d "$name" ]; then
        echo "ERROR: invalid cluster name"
        return 1
    fi

    rm -f "$name/envoy-values.yaml"
    KUBECONFIG="observability/kubeconfig.yaml" kubectl get secret -n $NAMESPACE "$name-envoy-values" -o json | jq -r '.data["values.yaml"]' | base64 -D > "$name/envoy-values.yaml"

    helm upgrade --kubeconfig "$name/kubeconfig.yaml" -i -n $NAMESPACE --values "$name/envoy-values.yaml" atlas-envoy ../../charts/envoy
}

function build_downstream_cluster() {
    local name=$1

    setup_downstream "$name"
    config_atlas_for_cluster "$name"
    config_cluster_envoy "$name"
}

function verify_variables() {
  if [ -z "$DIGITALOCEAN_ACCESS_TOKEN" ]; then
    echo "DIGITALOCEAN_ACCESS_TOKEN is required"
    exit 3
  fi

  if [ -z "$DIGITALOCEAN_SSH_KEYS" ]; then
    echo "DIGITALOCEAN_SSH_KEYS is required"
    exit 2
  fi
}

function details() {
  OBS_IP_ADDRESS=$(jq -r '.[0].networks.v4[0].ip_address' < "observability/droplet.json")
  DS1_IP_ADDRESS=$(jq -r '.[0].networks.v4[0].ip_address' < "downstream1/droplet.json")
  DS2_IP_ADDRESS=$(jq -r '.[0].networks.v4[0].ip_address' < "downstream2/droplet.json")
  DS3_IP_ADDRESS=$(jq -r '.[0].networks.v4[0].ip_address' < "downstream3/droplet.json")

  cat << EOF
IP Addresses
-----------------------------------------
Observability: $OBS_IP_ADDRESS
  Downstream1: $DS1_IP_ADDRESS
  Downstream2: $DS2_IP_ADDRESS
  Downstream3: $DS3_IP_ADDRESS

Observability Cluster
------------------------------------------
thanos-query: http://thanos-query.$OBS_IP_ADDRESS.nip.io
  prometheus: http://prometheus.$OBS_IP_ADDRESS.nip.io
      alerts: http://alerts.$OBS_IP_ADDRESS.nip.io

Accessing Downstream through Observability Cluster:

Note: these use the Envoy Proxy Network provided by Atlas to allow secure
communications to downstream cluster components.

 downstream1: http://thanos-query.$OBS_IP_ADDRESS.nip.io/prom/downstream1/graph
 downstream2: http://thanos-query.$OBS_IP_ADDRESS.nip.io/prom/downstream2/graph
 downstream3: http://thanos-query.$OBS_IP_ADDRESS.nip.io/prom/downstream3/graph

Important: In a real-world scenario you'd gate access to thanos-query via an oauth2 proxy
or it would only be accessible on an internal network!

=================================================================================
! IMPORTANT ! READ !
=================================================================================
The downstream cluster links are only available for DEMO purposes only! In a real
atlas deployment, the only ports that need to be accessible by the upstream
cluster are ports 11901-11904!
=================================================================================

Downstream Cluster 1
------------------------------------------
prometheus: http://prometheus.$DS1_IP_ADDRESS.nip.io
    alerts: http://alerts.$DS1_IP_ADDRESS.nip.io

Downstream Cluster 2
------------------------------------------
prometheus: http://prometheus.$DS2_IP_ADDRESS.nip.io
    alerts: http://alerts.$DS2_IP_ADDRESS.nip.io

Downstream Cluster 3
------------------------------------------
prometheus: http://prometheus.$DS3_IP_ADDRESS.nip.io
    alerts: http://alerts.$DS3_IP_ADDRESS.nip.io

EOF
}

if [ "$OP" == "details" ]; then
  details
  exit 0
fi

if [ "$OP" == "up" ]; then
    verify_variables
    setup_userdata
    setup_observability

    build_downstream_cluster downstream1
    build_downstream_cluster downstream2
    build_downstream_cluster downstream3

    details

    exit 0
fi

if [ "$OP" == "down" ]; then
    doctl compute droplet delete -f observability || echo "Instance does not exist or already deleted"
    rm -rf observability

    doctl compute droplet delete -f downstream1 || echo "Instance does not exist or already deleted"
    rm -rf downstream1

    doctl compute droplet delete -f downstream2 || echo "Instance does not exist or already deleted"
    rm -rf downstream2

    doctl compute droplet delete -f downstream3 || echo "Instance does not exist or already deleted"
    rm -rf downstream3

    rm -f userdata.yaml

    exit 0
fi

