# Getting Started

The easiest way to get started is if you are using the [kube-prometheus-stack]() helm chart already. The defaults for Atlas are based on how all the various compontents of prometheus are deployed including how the Thanos sidecar is deployed.

The rest of this guide will walk through configuring Atlas assuming you are using the [kube-prometheus-stack]() helm chart. If you are not, this guide should still be helpful but you'll have to modify values accordingly. Atlas has been designed to allow a considerable amount of control, but should you find something that cannot be modified please feel free to open a ticket to discuss.

!!! note
    If you want to do a quick start, you can use the `deploy.sh` script in the `hack/` directory to use Digital Ocean to spin up an observability cluster and 3 downstream clusters and automatically deploy Atlas, kube-prometheus-stack and everything else to have a complete working model to look at. The script takes about 5 minutes to run depending on a few different factors.

## Requirements

- 1 kubernetes cluster to act as the observability cluster
  - ability to modify `kube-system/coredns` configmap if you want thanos query to auto-discover sidecars
  - ability to install helm charts
  - inbound to ingress controller
  - inbound to port 10900 on host that has envoy proxy install
- 1 kubernetes cluster to act as a downstream cluster
  - ability to install helm charts

## Diving In

### Step 1. Install Atlas

There are a couple of helm values you'll need 