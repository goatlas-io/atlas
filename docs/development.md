# Development

Developing on Atlas is fairly simple but running all the various components to verify connectivity can be tough.

It is recommended that [k3d](https://k3d.io) be used which leverages [k3s](https://k3s.io).

## Steps

1. Create k3s Cluster `k3d cluster create atlas --api-port 192.168.11.103:6556` (subsitute the IP for the docker host IP)
2. Obtain k3s kubeconfig `k3d kubeconfig get atlas > atlas-kubeconfig.yaml`
3. Save the contents from step 2 to a file called `altas-kubeconfig.yaml`
4. Setup your terminal to use the file `export KUBECONFIG=./atlas-kubeconfig.yaml`
5. Now you are setup to do development, when you run atlas, it'll use the correct kubeconfig, alternatively you can specify it on the command line.
