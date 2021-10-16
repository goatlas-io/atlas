# Quick Start (aka Demo using Digital Ocean)

The easiest way to get started is to take the [deploy script](examples/demo-do/deploy.sh) for a spin. It requires a Digital Ocean account.

To use this script you'll need a Digital Ocean API token. Once you have acquired the token, simply export `DIGITALOCEAN_ACCESS_TOKEN` to your shell and then from the root of the Atlas project run the following ...

## Preflight

1. Obtain a Digital Ocean Access Token
2. Obtain the SSH Key ID of your SSH key in Digital Ocean
3. `export DIGITALOCEAN_ACCESS_TOKEN=<access-token>`
4. `export DIGITALOCEAN_SSH_KEYS=<key-id>`

!!! note
    I'd highly recommend the use of [direnv](https://direnv.net) for managing environment variables throughout directories.

## Deploy Time

```bash
bash examples/demo-do/deploy.sh up
```

!!! note
    This script takes approximately 5-7 minutes to run, depending on how fast Digital Ocean is. It's spinning up a total of 4 servers and installing [k3s](https://k3s.io), then using helm to install the necessary components like prometheus, thanos, envoy and atlas on the various servers.

This script will deploy four clusters:

- observability
- downstream1
- downstream2
- downstream3

Once the script is done running a set of details will be printed to the screen. If you want to see the details again simply re-run the script with `details` instead of `up`.

The details output will give you all the urls to the various components that can be interacted with on the observability cluster and the downstream clusters, see below for more details.

Generally speaking by the time the details page shows up downstream1 and downstream2 will already be connected. Downstream3 will still be in the process of coming online, but should only take another minute or two at most.

## Details

In general your details will look something like the following ...

```text
IP Addresses
-----------------------------------------
Observability: 143.198.182.161
  Downstream1: 198.211.117.92
  Downstream2: 143.244.174.92
  Downstream3: 137.184.97.135

Observability Cluster
------------------------------------------
thanos-query: http://thanos-query.143.198.182.161.nip.io
  prometheus: http://prometheus.143.198.182.161.nip.io
      alerts: http://alerts.143.198.182.161.nip.io

Accessing Downstream through Observability Cluster:

Note: these use the Envoy Proxy Network provided by Atlas to allow secure
communications to downstream cluster components.

 downstream1: http://thanos-query.143.198.182.161.nip.io/prom/downstream1/graph
 downstream2: http://thanos-query.143.198.182.161.nip.io/prom/downstream2/graph
 downstream3: http://thanos-query.143.198.182.161.nip.io/prom/downstream3/graph

Important: In a real-world scenario you'd gate access to thanos-query via an oauth2 proxy
or it would only be accessible on an internal network!
```

The link to thanos-query in the observability cluster is how you can see your thanos query connected to the sidecars.

The downstream1-3 links all use the thanos-query and the ingress path prefix that allows accessing of the downstream clusters from the observability cluster. You can confirm this by going to each link and pulling up the prometheus configuration, you'll see the external labels differ for each one.

## Cleanup

When you are all done, `bash examples/demo-do/depoy.sh down` to tear it all down.
