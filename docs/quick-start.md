# Quick Start

The easiest way to get started is to take the [deploy script](hack/deployment/deploy.sh) for a spin. It requires a Digital Ocean account.

To use this script you'll need a Digital Ocean API token. Once you have acquired the token, simply export `DIGITALOCEAN_ACCESS_TOKEN` to your shell and then from the root of the Atlas project run the following ...

```bash
bash hack/deployment/deploy.sh up
```

This script will deploy four clusters:

- observability
- downstream1
- downstream2
- downstream3

Once the script is done running a set of details will be printed to the screen. If you want to see the details again simply re-run the script with `down` instead of `up`.

When you are all done, `bash hack/deployment/depoy.sh down` to tear it all down.
