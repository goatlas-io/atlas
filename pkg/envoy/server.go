package envoy

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/bwmarrin/snowflake"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"

	"github.com/ekristen/atlas/pkg/common"
	"github.com/ekristen/atlas/pkg/config"

	"github.com/rancher/wrangler/pkg/apply"
	wranglercorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"

	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	runtimeservice "github.com/envoyproxy/go-control-plane/envoy/service/runtime/v3"
	secretservice "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"

	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	server "github.com/envoyproxy/go-control-plane/pkg/server/v3"

	k8scorev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

type atlasCluster struct {
	Name      string
	Namespace string
	Replicas  int
	IP        string

	ThanosService string
	ThanosPort    uint32

	PrometheusService string
	PromPort          uint32

	AlertManagerService string
	AMPort              uint32

	service *k8scorev1.Service
}

type EnvoyADS struct {
	lock                     sync.Mutex
	config                   *config.EnvoyADSConfig
	log                      *logrus.Entry
	apply                    apply.Apply
	cli                      *cli.Context
	callbacks                *Callbacks
	cache                    cache.SnapshotCache
	server                   server.Server
	node                     *snowflake.Node
	debugEnvoy               bool
	grpcMaxConcurrentStreams uint32

	services      wranglercorev1.ServiceController
	servicesCache wranglercorev1.ServiceCache
	secrets       wranglercorev1.SecretController
	secretsCache  wranglercorev1.SecretCache
}

func Register(
	ctx context.Context,
	config *config.EnvoyADSConfig,
	log *logrus.Entry,
	apply apply.Apply,
	cliCtx *cli.Context,
	services wranglercorev1.ServiceController,
	secrets wranglercorev1.SecretController) *EnvoyADS {

	ads := &EnvoyADS{
		grpcMaxConcurrentStreams: 1000000,
		services:                 services,
		servicesCache:            services.Cache(),
		secrets:                  secrets,
		secretsCache:             secrets.Cache(),
		config:                   config,
		log:                      log,
		apply:                    apply,
		cli:                      cliCtx,
		debugEnvoy:               false,
	}

	return ads
}

func (e *EnvoyADS) Start(ctx context.Context, port int, instance int64, debugEnvoy bool) error {
	node, err := snowflake.NewNode(instance)
	if err != nil {
		return err
	}
	e.node = node

	cb := &Callbacks{
		log: e.log.WithField("component", "ads-callbacks"),
	}

	e.cache = cache.NewSnapshotCache(true, cache.IDHash{}, nil)
	e.server = server.NewServer(ctx, e.cache, cb)
	e.debugEnvoy = debugEnvoy

	e.log.Info("Starting Envoy ADS Server")

	if err := e.Sync(); err != nil {
		e.log.WithError(err).Error("unable to sync")
	}

	e.secrets.OnChange(ctx, "envoy-ads", e.secretOnChange)
	e.services.OnChange(ctx, "envoy-ads", e.serviceOnChange)

	go e.RunServer(ctx, e.log, e.server, port)

	<-ctx.Done()

	e.log.Info("Shutting down Envoy ADS Server")

	return nil
}

func (e *EnvoyADS) secretOnChange(key string, secret *k8scorev1.Secret) (*k8scorev1.Secret, error) {
	if secret == nil {
		return nil, nil
	}

	labels := secret.GetLabels()
	if _, ok := labels[common.IsCALabel]; ok {
		if err := e.Sync(); err != nil {
			e.log.WithError(err).Error("unable to sync resources")
		}
	}

	if _, ok := labels[common.IsCertLabel]; ok {
		if err := e.Sync(); err != nil {
			e.log.WithError(err).Error("unable to sync resources")
		}
	}

	return secret, nil
}

func (e *EnvoyADS) serviceOnChange(key string, service *k8scorev1.Service) (*k8scorev1.Service, error) {
	if service == nil {
		if strings.Contains(key, common.MonitoringNamespace) {
			if err := e.Sync(); err != nil {
				e.log.WithError(err).Error("unable to sync")
			}
		}

		return nil, nil
	}

	labels := service.GetLabels()
	if _, ok := labels[common.AtlasClusterLabel]; ok {
		if err := e.Sync(); err != nil {
			e.log.WithError(err).Error("unable to sync")
		}
	}

	return service, nil
}

func (e *EnvoyADS) Sync() error {
	e.lock.Lock()
	defer e.lock.Unlock()

	clusters, err := e.getClusters()
	if err != nil {
		return err
	}

	versionID := fmt.Sprintf("v.%d", e.node.Generate())

	if err := e.SyncObservability(versionID, clusters); err != nil {
		return err
	}

	if err := e.SyncClusters(versionID, clusters); err != nil {
		return err
	}

	return nil
}

func (e *EnvoyADS) SyncClusters(versionID string, clusters []*atlasCluster) error {
	actualAMServices := []*k8scorev1.Service{}

	ca, err := e.secretsCache.Get(common.MonitoringNamespace, common.CASecretName)
	if err != nil {
		return err
	}

	server, err := e.secretsCache.Get(common.MonitoringNamespace, common.ServerSecretName)
	if err != nil {
		return err
	}

	client, err := e.secretsCache.Get(common.MonitoringNamespace, common.ClientSecretName)
	if err != nil {
		return err
	}

	amServices, err := e.services.List(common.MonitoringNamespace, v1.ListOptions{
		LabelSelector: e.cli.String("alertmanager-selector"),
	})
	if err != nil {
		return err
	}

	for _, service := range amServices.Items {
		if _, ok := service.Spec.Selector["statefulset.kubernetes.io/pod-name"]; ok {
			actualAMServices = append(actualAMServices, &service)
		}
	}

	for _, cluster := range clusters {
		sidecarVirtualHost := buildVirtualHost("thanos_sidecar", []string{"*"}, "thanos_sidecar", "/", "", nil, false)
		prometheusVirtualHost := buildVirtualHost("prometheus", []string{"*"}, "prometheus", "/", "", nil, false)

		// Note: these listeners are connected to from the Observerability Cluster Envoy Proxy
		dsclusterListeners := []types.Resource{
			buildListener("thanos_sidecar", common.ClusterInboundThanosPort, "thanos_sidecar", "server", true),
			buildListener("prometheus", common.ClusterInboundPrometheusPort, "prometheus", "server", true),
			// TODO: add alertmanager
		}

		// Note: these are cluster definitions for the downstream envoy proxy of services that define local services
		// that are targets of connections
		dsclusterClusters := []types.Resource{
			buildCluster("thanos_sidecar", cluster.ThanosService, common.ObservabilityThanosPort, false, true),
			buildCluster("prometheus", cluster.PrometheusService, common.PrometheusPort, false, false),
			// TODO: add alertmanager
		}

		// Note: this is the virtualhost definitions for the listener for the sidecar to ensure
		// routing to the sidecar happens properly
		dsclusterRoutes := []types.Resource{
			buildRouteRaw("thanos_sidecar", []*route.VirtualHost{sidecarVirtualHost}),
			buildRouteRaw("prometheus", []*route.VirtualHost{prometheusVirtualHost}),
			// TODO: add alertmanager
		}

		// Note: we do not send the client cert, because the is controlled by the
		// static cluster definition for the xds_cluster for dynamic discovery.
		dsclusterSecretResources := []types.Resource{
			buildSecretTLSValidation("validation", combineCAs(ca)),
			buildSecretTLSCertificate("server", server.Data["tls.crt"], server.Data["tls.key"]),
		}

		// If there are alertmanagers deployed, modify the the downstream cluster ADS configuration appropriately
		if len(actualAMServices) > 0 && "localhost" != e.config.AtlasEnvoyAddress {
			dsclusterClusters = append(dsclusterClusters, buildCluster("alertmanagers", e.config.AtlasEnvoyAddress, common.ObservabilityAlertManagerPort, true, true))

			// Note: no secret is passed so it listens WITHOUT https since it's all local
			dsclusterListeners = append(dsclusterListeners, buildListener("alertmanagers", common.ClusterInboundAlertManagerPort, "alertmanagers", "", false))

			amVirtualhosts := []*route.VirtualHost{}

			for i := range actualAMServices {
				name := fmt.Sprintf("alertmanager%d", i)
				domains := []string{
					// Build the alertmanager domains, alertmanagerN.monitoring.svc.cluster.local
					// this is on the local downstream cluster envoy, but will be fowarded to the Observability Cluster envoy and
					// rewritten to the service DNS name for the alertmanager replica that matches the N value.
					fmt.Sprintf("alertmanager%d.%s.svc.cluster.local*", i, common.MonitoringNamespace),
				}
				amVirtualhosts = append(amVirtualhosts, buildVirtualHost(name, domains, "alertmanagers", "/", "", nil, false))
			}

			dsclusterRoutes = append(dsclusterRoutes, buildRouteRaw("alertmanagers", amVirtualhosts))

			dsclusterSecretResources = append(dsclusterSecretResources, buildSecretTLSCertificate("client", client.Data["tls.crt"], client.Data["tls.key"]))
		}

		dsclusterSnapshot := cache.NewSnapshot(
			versionID,
			[]types.Resource{},       // endpoints
			dsclusterClusters,        // clusters
			dsclusterRoutes,          // routes
			dsclusterListeners,       // listeners
			[]types.Resource{},       // runtimes
			dsclusterSecretResources, // secrets
		)

		slog := e.log.WithField("id", cluster.Name).WithField("version", versionID)
		slog.Info("generating snapshot ", versionID)
		if err := dsclusterSnapshot.Consistent(); err != nil {
			slog.WithError(err).Error("snapshot inconsistency")
			return err
		}

		if err := e.cache.SetSnapshot(cluster.Name, dsclusterSnapshot); err != nil {
			slog.WithError(err).Error("snapshot error")
			return err
		}

		snapshots.Inc()
	}

	return nil
}

func (e *EnvoyADS) SyncObservability(versionID string, clusters []*atlasCluster) error {
	addClientSecret := false

	if len(clusters) > 0 {
		addClientSecret = true
	}

	ca, err := e.secretsCache.Get(common.MonitoringNamespace, common.CASecretName)
	if err != nil {
		return err
	}

	server, err := e.secretsCache.Get(common.MonitoringNamespace, common.ServerSecretName)
	if err != nil {
		return err
	}

	client, err := e.secretsCache.Get(common.MonitoringNamespace, common.ClientSecretName)
	if err != nil {
		return err
	}

	secretResources := []types.Resource{
		buildSecretTLSValidation("validation", combineCAs(ca)),
		buildSecretTLSCertificate("server", server.Data["tls.crt"], server.Data["tls.key"]),
	}

	if addClientSecret {
		secretResources = append(secretResources, buildSecretTLSCertificate("client", client.Data["tls.crt"], client.Data["tls.key"]))
	}

	clusterResources := []types.Resource{}
	virtualhosts := []*route.VirtualHost{}
	actualAMServices := []*k8scorev1.Service{}

	amServices, err := e.services.List(common.MonitoringNamespace, v1.ListOptions{
		LabelSelector: e.cli.String("alertmanager-selector"),
	})
	if err != nil {
		return err
	}

	for _, service := range amServices.Items {
		if _, ok := service.Spec.Selector["statefulset.kubernetes.io/pod-name"]; ok {
			actualAMServices = append(actualAMServices, &service)
		}
	}

	for i, service := range actualAMServices {
		name := fmt.Sprintf("alertmanager%d", i)
		fqdn := fmt.Sprintf("%s.%s.svc.cluster.local", service.GetName(), service.GetNamespace())

		clusterResources = append(clusterResources, buildCluster(name, fqdn, common.AlertManagerPort, false, false))
	}

	if e.debugEnvoy {
		clusterResources = append(clusterResources, buildCluster("envoy_proxy", "www.envoyproxy.io", 80, false, true))
		clusterResources = append(clusterResources, buildCluster("google", "www.google.com", 80, false, true))
	}

	promDomains := []string{"*"}
	promVhRoutes := []*route.Route{}

	for _, r := range clusters {
		thanosName := fmt.Sprintf("%s-thanos", r.Name)
		promName := fmt.Sprintf("%s-prom", r.Name)

		clusterResources = append(clusterResources, buildCluster(thanosName, r.IP, r.ThanosPort, true, true))
		clusterResources = append(clusterResources, buildCluster(promName, r.IP, r.PromPort, true, true))

		domains := []string{
			fmt.Sprintf("%s.%s.svc.cluster.local*", r.Name, r.Name),
		}

		for i := 0; i < r.Replicas; i++ {
			domains = append(domains, fmt.Sprintf("%s-thanos-sidecar%d.%s.svc.cluster.local*", r.Name, i, r.Namespace))
		}

		rewrite := r.PrometheusService
		virtualhosts = append(virtualhosts, buildVirtualHost(thanosName, domains, thanosName, "/", rewrite, nil, false))

		prefixParts := []string{"prom", r.Name}
		prefix := strings.Join(prefixParts, "/")

		promVhRoutes = append(promVhRoutes, buildVirtualHostRoute(fmt.Sprintf("/%s/", prefix), promName, "", &[]string{"/"}[0], false))
	}

	promVH := &route.VirtualHost{
		Name:    "prometheus",
		Domains: promDomains,
		Routes:  promVhRoutes,
	}

	routeResources := []types.Resource{
		buildRoute("xds_local", "xds_cluster", "localhost"),
		buildRouteRaw("downstream_thanos", virtualhosts),
		buildRouteRaw("downstream_prometheus", []*route.VirtualHost{promVH}),
	}

	if len(actualAMServices) > 0 {
		amVirtualhosts := []*route.VirtualHost{}

		for i, service := range actualAMServices {
			name := fmt.Sprintf("alertmanager%d", i)
			domains := []string{
				// Build the alertmanager domains, alertmanagerN.monitoring.svc.cluster.local
				// this is the incoming domain from the Downstream Cluster, it gets rewritten to the actual
				// service DNS name for the alertmanager replica that matches the N value.
				fmt.Sprintf("alertmanager%d.%s.svc.cluster.local*", i, common.MonitoringNamespace),
			}

			rewrite := fmt.Sprintf("%s.%s.svc.cluster.local:9093", service.GetName(), service.GetNamespace())
			amVirtualhosts = append(amVirtualhosts, buildVirtualHost(name, domains, name, "/", rewrite, nil, false))
		}

		routeResources = append(routeResources, buildRouteRaw("upstream_alertmanagers", amVirtualhosts))
	}

	if e.debugEnvoy {
		routeResources = append(routeResources, buildRoute("envoy_route", "envoy_proxy", "www.envoyproxy.io"))
		routeResources = append(routeResources, buildRoute("google_route", "google", "www.google.com"))
	}

	listenerResources := []types.Resource{
		buildListener("xds_external", common.ObservabilityADSPort, "xds_local", "server", false),                       // 10900
		buildListener("downstream_thanos", common.ObservabilityThanosPort, "downstream_thanos", "", false),             // 10901
		buildListener("downstream_prometheus", common.ObservabilityPrometheusPort, "downstream_prometheus", "", false), // 10904
	}

	if len(actualAMServices) > 0 {
		listenerResources = append(listenerResources, buildListener("upstream_alertmanagers", common.ObservabilityAlertManagerPort, "upstream_alertmanagers", "server", true)) // 10903
	}

	if e.debugEnvoy {
		listenerResources = append(listenerResources, buildListener("envoyproxy", 10000, "envoy_route", "server", false))
		listenerResources = append(listenerResources, buildListener("google", 10001, "google_route", "server", false))
	}

	snapshot := cache.NewSnapshot(
		versionID,
		[]types.Resource{}, // endpoints
		clusterResources,   // clusters
		routeResources,     // routes
		listenerResources,  // listeners
		[]types.Resource{}, // runtimes
		secretResources,    // secrets
	)

	slog := e.log.WithField("id", common.EnvoyADSObservabilityID).WithField("version", versionID)
	slog.Info("generating snapshot ", versionID)
	if err := snapshot.Consistent(); err != nil {
		slog.WithError(err).Errorf("snapshot inconsistency")
		return err
	}

	if err := e.cache.SetSnapshot(common.EnvoyADSObservabilityID, snapshot); err != nil {
		slog.WithError(err).Error("unable to set snapshot")
		return err
	}

	snapshots.Inc()

	return nil
}

// RunManagementServer starts an xDS server at the given port.
func (e *EnvoyADS) RunServer(ctx context.Context, log *logrus.Entry, server server.Server, port int) {
	var grpcOptions []grpc.ServerOption
	grpcOptions = append(grpcOptions, grpc.MaxConcurrentStreams(e.grpcMaxConcurrentStreams))
	grpcServer := grpc.NewServer(grpcOptions...)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.WithError(err).Fatal("Unable to start grpc network listener")
	}

	registerServer(grpcServer, server)

	log.WithFields(logrus.Fields{"port": port}).Info("Starting Envoy ADS GRPC Management Server")

	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.WithError(err).Error("Unable to start grpc server")
		}
	}()

	<-ctx.Done()

	log.Info("Shutting down Envoy ADS GRPC Management Server")

	grpcServer.GracefulStop()
}

func registerServer(grpcServer *grpc.Server, server server.Server) {
	// register services
	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, server)
	endpointservice.RegisterEndpointDiscoveryServiceServer(grpcServer, server)
	clusterservice.RegisterClusterDiscoveryServiceServer(grpcServer, server)
	routeservice.RegisterRouteDiscoveryServiceServer(grpcServer, server)
	listenerservice.RegisterListenerDiscoveryServiceServer(grpcServer, server)
	secretservice.RegisterSecretDiscoveryServiceServer(grpcServer, server)
	runtimeservice.RegisterRuntimeDiscoveryServiceServer(grpcServer, server)
}

func (e *EnvoyADS) getClusters() ([]*atlasCluster, error) {
	requirement, err := labels.NewRequirement(common.AtlasClusterLabel, selection.Exists, []string{})
	if err != nil {
		return nil, err
	}
	selector := labels.NewSelector().Add(*requirement)

	services, err := e.servicesCache.List(common.MonitoringNamespace, selector)
	if err != nil {
		return nil, err
	}

	clusters := []*atlasCluster{}

	for _, s := range services {
		annotations := s.GetAnnotations()
		labels := s.GetLabels()

		replicas := 1
		if v, ok := labels[common.ReplicasLabel]; ok {
			i, err := strconv.Atoi(v)
			if err != nil {
				logrus.WithError(err).Error("unable to convert string to int")
			} else {
				replicas = i
			}
		}

		thanosService := common.ThanosFQDN
		if v, ok := annotations[common.ThanosServiceLabel]; ok {
			thanosService = v
		}

		prometheusService := common.PrometheusFQDN
		if v, ok := annotations[common.PrometheusServiceLabel]; ok {
			prometheusService = v
		}

		clusters = append(clusters, &atlasCluster{
			Name:       s.Name,
			Namespace:  s.Namespace,
			Replicas:   replicas,
			IP:         s.Spec.ExternalIPs[0],
			ThanosPort: uint32(common.ClusterInboundThanosPort),
			PromPort:   uint32(common.ClusterInboundPrometheusPort),
			AMPort:     uint32(common.ClusterInboundAlertManagerPort),
			service:    s,

			ThanosService:     thanosService,
			PrometheusService: prometheusService,
		})
	}

	return clusters, nil
}
