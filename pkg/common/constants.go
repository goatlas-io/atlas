package common

const (
	MonitoringNamespace = "monitoring"

	CASecretName         = "atlas-ca"
	ServerSecretName     = "atlas-server"
	ClientSecretName     = "atlas-client"
	IngressTLSSecretName = "atlas-tls"

	CAOwnerID          = "atlas-ca"
	CARotateAnnotation = "goatlas.io/ca-rotate"
	CARevisionLabel    = "goatlas.io/ca-revision"
	CASerialLabel      = "goatlas.io/ca-serial"
	CASignedSerial     = "goatlas.io/ca-signed"
	CAChecksumLabel    = "goatlas.io/ca-checksum"
	CAUsageClientLabel = "goatlas.io/ca-usage-client"
	CAUsageServerLabel = "goatlas.io/ca-usage-server"
	IsCALabel          = "goatlas.io/ca"
	IsCertLabel        = "goatlas.io/cert"

	AtlasClusterLabel = "goatlas.io/cluster"
	SidecarLabel      = "goatlas.io/thanos-sidecar"
	ReplicasLabel     = "goatlas.io/replicas"

	EnvoySelectorsAnnotation = "goatlas.io/envoy-selectors"
	EnvoySelectors           = "app=envoy,release=atlas"

	// These are uses on a service to change the default service fqdn from the downstream
	// cluster. This is mainly useful when the prometheus-operator is not being used.
	ThanosServiceAnnotation           = "goatlas.io/thanos-service"
	ThanosServicePortAnnotation       = "goatlas.io/thanos-service-port"
	PrometheusServiceAnnotation       = "goatlas.io/prometheus-service"
	PrometheusServicePortAnnotation   = "goatlas.io/prometheus-service-port"
	AlertManagerServiceAnnotation     = "goatlas.io/alertmanager-service"
	AlertManagerServicePortAnnotation = "goatlas.io/alertmanager-service-port"

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
