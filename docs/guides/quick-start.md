# Quick Start

If you want to just take Atlas for a spin there exists [a script](hack/deployment/deploy.sh) that when combined with a Digital Ocean API token can spin up a fully working observability cluster, plus 3 downstream clusters and demonstrate the Atlas first hand.

To get started you'll need a Digital Ocean API token and the Digital Ocean CLI tool. Once you have the CLI installed, please export your token as an environment variable in your shell like `export DIGITALOCEAN_ACCESS_TOKEN=<token>`. Finally you may begin by running `bash hack/deployment/deploy.sh up` (Note: the script will change directory to the directory in which the script resides and use that directory to place temporary files for managing the deployment)

When you are all done, `bash hack/deployment/depoy.sh down` to tear it all down.

At the end of the `up` run, the script will output a bunch of information for you to try out the range.

## Configuration

There are some customizations you can make to the script using environment variables, simple export the value you want to your shell before you run the script.

```bash
DO_REGION="nyc1"
DO_SIZE="s-2vcpu-4gb"
DO_IMAGE="ubuntu-20-04-x64"

HELM_TAG="beta1-201cc43"
HELM_PULLSECRET="github"

NAMESPACE="monitoring"
```
