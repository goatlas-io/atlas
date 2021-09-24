package envoy

import (
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	tls "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/ptypes"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func buildCluster(clusterName, upstreamHost string, upstreamPort uint32, upstreamTLS bool, http2 bool) *cluster.Cluster {
	cluster := &cluster.Cluster{
		Name:                 clusterName,
		ConnectTimeout:       ptypes.DurationProto(5 * time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_LOGICAL_DNS},
		LbPolicy:             cluster.Cluster_ROUND_ROBIN,
		LoadAssignment:       buildEndpoint(clusterName, upstreamHost, upstreamPort),
		DnsLookupFamily:      cluster.Cluster_V4_ONLY,
	}

	if http2 {
		cluster.Http2ProtocolOptions = &core.Http2ProtocolOptions{}
	}

	if upstreamTLS {
		uTLS := buildUpstreamTLS("client")

		tctx, err := ptypes.MarshalAny(uTLS)
		if err != nil {
			panic(err)
		}

		cluster.TransportSocket = &core.TransportSocket{
			Name: "envoy.transport_sockets.tls",
			ConfigType: &core.TransportSocket_TypedConfig{
				TypedConfig: tctx,
			},
		}
	}

	return cluster
}

func buildEndpoint(clusterName string, upstreamHost string, upstreamPort uint32) *endpoint.ClusterLoadAssignment {
	return &endpoint.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints: []*endpoint.LocalityLbEndpoints{{
			LbEndpoints: []*endpoint.LbEndpoint{{
				HostIdentifier: &endpoint.LbEndpoint_Endpoint{
					Endpoint: &endpoint.Endpoint{
						Address: &core.Address{
							Address: &core.Address_SocketAddress{
								SocketAddress: &core.SocketAddress{
									Protocol: core.SocketAddress_TCP,
									Address:  upstreamHost,
									PortSpecifier: &core.SocketAddress_PortValue{
										PortValue: upstreamPort,
									},
								},
							},
						},
					},
				},
			}},
		}},
	}
}

func buildRoute(routeName string, clusterName string, rewriteHost string) *route.RouteConfiguration {
	return &route.RouteConfiguration{
		Name: routeName,
		VirtualHosts: []*route.VirtualHost{
			{
				Name:    "backend",
				Domains: []string{"*"},
				Routes: []*route.Route{
					{
						Match: &route.RouteMatch{
							PathSpecifier: &route.RouteMatch_Prefix{
								Prefix: "/",
							},
						},
						Action: &route.Route_Route{
							Route: &route.RouteAction{
								ClusterSpecifier: &route.RouteAction_Cluster{
									Cluster: clusterName,
								},
								HostRewriteSpecifier: &route.RouteAction_HostRewriteLiteral{
									HostRewriteLiteral: rewriteHost,
								},
							},
						},
					},
				},
			},
		},
	}
}

func buildRouteRaw(routeName string, virtualhosts []*route.VirtualHost) *route.RouteConfiguration {
	return &route.RouteConfiguration{
		Name:         routeName,
		VirtualHosts: virtualhosts,
	}
}

func buildVirtualHostRoute(pathPrefix string, clusterName string, rewriteHost string, rewritePrefix *string, matchGRPC bool) *route.Route {
	r := &route.Route{
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{
				Prefix: pathPrefix,
			},
		},
	}

	routeAction := &route.RouteAction{
		ClusterSpecifier: &route.RouteAction_Cluster{
			Cluster: clusterName,
		},
	}

	if rewriteHost != "" {
		routeAction.HostRewriteSpecifier = &route.RouteAction_HostRewriteLiteral{
			HostRewriteLiteral: rewriteHost,
		}
	}

	if rewritePrefix != nil {
		routeAction.PrefixRewrite = *rewritePrefix
	}

	r.Action = &route.Route_Route{
		Route: routeAction,
	}

	if matchGRPC {
		r.Match.Grpc = &route.RouteMatch_GrpcRouteMatchOptions{}
	}

	return r
}

func buildVirtualHost(name string, domains []string, clusterName string, pathPrefix string, rewriteHost string, rewritePrefix *string, grpc bool) *route.VirtualHost {
	vh := &route.VirtualHost{
		Name:    name,
		Domains: domains,
		Routes: []*route.Route{
			buildVirtualHostRoute(pathPrefix, clusterName, rewriteHost, rewritePrefix, grpc),
		},
	}

	return vh
}

func buildListener(listenerName string, listenerPort uint32, route string, secretName string, clientValidation bool) *listener.Listener {
	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.HttpConnectionManager_AUTO,
		StatPrefix: "http",
		RouteSpecifier: &hcm.HttpConnectionManager_Rds{
			Rds: &hcm.Rds{
				ConfigSource:    buildConfigSource(),
				RouteConfigName: route,
			},
		},

		HttpFilters: []*hcm.HttpFilter{{
			Name: wellknown.Router,
		}},
	}

	pbst, err := ptypes.MarshalAny(manager)
	if err != nil {
		panic(err)
	}

	filterChain := &listener.FilterChain{
		Filters: []*listener.Filter{
			{
				Name: wellknown.HTTPConnectionManager,
				ConfigType: &listener.Filter_TypedConfig{
					TypedConfig: pbst,
				},
			},
		},
	}

	if secretName != "" {
		tlsContext := buildDownstreamTLS(secretName, clientValidation)
		scfg, err := ptypes.MarshalAny(tlsContext)
		if err != nil {
			panic(err)
		}

		filterChain.TransportSocket = &core.TransportSocket{
			Name: "envoy.transport_sockets.tls",
			ConfigType: &core.TransportSocket_TypedConfig{
				TypedConfig: scfg,
			},
		}
	}

	listener := &listener.Listener{
		Name: listenerName,
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.SocketAddress_TCP,
					Address:  "0.0.0.0",
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: listenerPort,
					},
				},
			},
		},
		FilterChains: []*listener.FilterChain{
			filterChain,
		},
	}

	return listener
}

func buildConfigSource() *core.ConfigSource {
	source := &core.ConfigSource{}
	source.ResourceApiVersion = resource.DefaultAPIVersion

	source.ConfigSourceSpecifier = &core.ConfigSource_Ads{
		Ads: &core.AggregatedConfigSource{},
	}

	return source
}

func buildDownstreamTLS(secretName string, clientValidation bool) *tlsv3.DownstreamTlsContext {
	downstreamTLS := &tlsv3.DownstreamTlsContext{
		RequireClientCertificate: &wrapperspb.BoolValue{Value: clientValidation},
		CommonTlsContext: &tlsv3.CommonTlsContext{
			AlpnProtocols: []string{"h2", "http/1.1"},
			TlsCertificateSdsSecretConfigs: []*tlsv3.SdsSecretConfig{
				{
					Name: secretName,
					SdsConfig: &core.ConfigSource{
						ConfigSourceSpecifier: &core.ConfigSource_Ads{
							Ads: &core.AggregatedConfigSource{},
						},
						ResourceApiVersion: core.ApiVersion_V3,
					},
				},
			},
			ValidationContextType: &tlsv3.CommonTlsContext_ValidationContextSdsSecretConfig{
				ValidationContextSdsSecretConfig: &tlsv3.SdsSecretConfig{
					Name: "validation",
					SdsConfig: &core.ConfigSource{
						ConfigSourceSpecifier: &core.ConfigSource_Ads{
							Ads: &core.AggregatedConfigSource{},
						},
						ResourceApiVersion: core.ApiVersion_V3,
					},
				},
			},
		},
	}

	return downstreamTLS
}

func buildUpstreamTLS(secretName string) *tls.UpstreamTlsContext {
	return &tls.UpstreamTlsContext{
		CommonTlsContext: &tls.CommonTlsContext{
			AlpnProtocols: []string{"h2", "http/1.1"},
			TlsCertificateSdsSecretConfigs: []*tls.SdsSecretConfig{
				{
					Name: secretName,
					SdsConfig: &core.ConfigSource{
						ConfigSourceSpecifier: &core.ConfigSource_Ads{
							Ads: &core.AggregatedConfigSource{},
						},
						ResourceApiVersion: core.ApiVersion_V3,
					},
				},
			},
			ValidationContextType: &tls.CommonTlsContext_ValidationContextSdsSecretConfig{
				ValidationContextSdsSecretConfig: &tls.SdsSecretConfig{
					Name: "validation",
					SdsConfig: &core.ConfigSource{
						ConfigSourceSpecifier: &core.ConfigSource_Ads{
							Ads: &core.AggregatedConfigSource{},
						},
						ResourceApiVersion: core.ApiVersion_V3,
					},
				},
			},
		},
	}
}

func buildSecretTLSCertificate(name string, cert, key []byte) *tls.Secret {
	return &tls.Secret{
		Name: name,
		Type: &tls.Secret_TlsCertificate{
			TlsCertificate: &tls.TlsCertificate{
				CertificateChain: &core.DataSource{
					Specifier: &core.DataSource_InlineBytes{InlineBytes: []byte(cert)},
				},
				PrivateKey: &core.DataSource{
					Specifier: &core.DataSource_InlineBytes{InlineBytes: []byte(key)},
				},
			},
		},
	}
}

func buildSecretTLSValidation(name string, ca []byte) *tls.Secret {
	return &tls.Secret{
		Name: name,
		Type: &tls.Secret_ValidationContext{
			ValidationContext: &tls.CertificateValidationContext{
				TrustedCa: &core.DataSource{
					Specifier: &core.DataSource_InlineBytes{InlineBytes: []byte(ca)},
				},
			},
		},
	}
}
