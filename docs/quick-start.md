# Quick Start (aka Demo using Digital Ocean)

The easiest way to get started is to take the [deploy script](examples/demo-do/deploy.sh) for a spin. It requires a Digital Ocean account.

To use this script you'll need a Digital Ocean API token. Once you have acquired the token, simply export `DIGITALOCEAN_ACCESS_TOKEN` to your shell and then from the root of the Atlas project run the following ...

```bash
bash examples/demo-do/deploy.sh up
```

This script will deploy four clusters:

- observability
- downstream1
- downstream2
- downstream3

Once the script is done running a set of details will be printed to the screen. If you want to see the details again simply re-run the script with `details` instead of `up`.

The details output will give you all the urls to the various components that can be interacted with on the observability cluster and the downstream clusters.

When you are all done, `bash examples/demo-do/depoy.sh down` to tear it all down.
