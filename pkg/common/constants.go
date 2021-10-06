package common

const (
	MonitoringNamespace = "monitoring"

	CASecretName         = "atlas-ca"
	ServerSecretName     = "atlas-server"
	ClientSecretName     = "atlas-client"
	IngressTLSSecretName = "atlas-tls"

	CAOwnerID          = "atlas-ca"
	CARotateAnnotation = "atlas.ekristen.github.com/ca-rotate"
	CARevisionLabel    = "atlas.ekristen.github.com/ca-revision"
	CASerialLabel      = "atlas.ekristen.github.com/ca-serial"
	CASignedSerial     = "atlas.ekristen.github.com/ca-signed"
	CAChecksumLabel    = "atlas.ekristen.github.com/ca-checksum"
	CAUsageClientLabel = "atlas.ekristen.github.com/ca-usage-client"
	CAUsageServerLabel = "atlas.ekristen.github.com/ca-usage-server"
	IsCALabel          = "atlas.ekristen.github.com/ca"
	IsCertLabel        = "atlas.ekristen.github.com/cert"

	ThanosClusterLabel = "atlas.ekristen.github.com/atlas"
	SidecarLabel       = "atlas.ekristen.github.com/thanos-sidecar"
	ReplicasLabel      = "atlas.ekristen.github.com/replicas"

	EnvoySelectorsAnnotation = "atlas.ekristen.github.com/envoy-selectors"
	EnvoySelectors           = "app=envoy,release=atlas"

	// These are uses on a service to change the default service fqdn from the downstream
	// cluster. This is mainly useful when the prometheus-operator is not being used.
	ThanosServiceLabel       = "atlas.ekristen.github.com/thanos-service"
	PrometheusServiceLabel   = "atlas.ekristen.github.com/prometheus-service"
	AlertManagerServiceLabel = "atlas.ekristen.github.com/alertmanager-service"

	ThanosFQDN       = "prometheus-operated.monitoring.svc.cluster.local"
	ThanosPort       = 10901
	PrometheusFQDN   = "prometheus-operated.monitoring.svc.cluster.local"
	PrometheusPort   = 9090
	AlertManagerFQDN = "alertmanager-operated.monitoring.svc.cluster.local"
	AlertManagerPort = 9093

	ClusterInboundThanosPort       = 11901 // This is the port envoy listens to on the downstream cluster for connections to thanos
	ClusterInboundPrometheusPort   = 11904 // This is the port envoy listens to on the downstream cluster for connections to prometheus
	ClusterInboundAlertManagerPort = 11903 // This is the port envoy listens to on the downstream cluster for connections to alertmanager

	ObservabilityADSPort          = 10900
	ObservabilityThanosPort       = 10901
	ObservabilityPrometheusPort   = 10904
	ObservabilityAlertManagerPort = 10903

	ObservabilityAlertManagerServiceLabel = "app=kube-prometheus-stack-alertmanager"

	ObservabilityEnvoyAtlasOwnerID     = "atlas-envoy"
	ObservabilityEnvoyValuesSecretName = "atlas-envoy-values"

	DNSOwnerID       = "atlas-dns"
	DNSConfigMapName = "atlas-coredns"
	DNSTLD           = "atlas"

	EnvoyADSObservabilityID = "atlas"
	EnvoyADSClusterID       = "cluster"
)
