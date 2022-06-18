# Envoy Chart for Atlas

This chart was originally forked from the stable/envoy chart, but has been slightly modified to add some specific Atlas related configurations that compliment the Envoy deployment.

## Modifications

- Atlas Additional Alertmanager Configuration -- based on the number of alertmanagers deployed in the observability cluster, this is automatically configured.
- Atlas Alertmanager Services -- based on the number of alertmanagers deployed in the observability cluster, this is automatically configured.
