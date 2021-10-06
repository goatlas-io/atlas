package commands

import (
	"context"

	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/leader"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/rancher/wrangler/pkg/start"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"

	"github.com/ekristen/atlas/pkg/common"
	"github.com/ekristen/atlas/pkg/config"
	"github.com/ekristen/atlas/pkg/controllers/atlas"
	"github.com/ekristen/atlas/pkg/metrics"
)

type controlCommand struct{}

func (s *controlCommand) Execute(c *cli.Context) error {
	// set up signals so we handle the first shutdown signal gracefully
	ctx := signals.SetupSignalHandler(context.Background())

	log := logrus.WithField("command", "controller")

	go metrics.NewMetricsServer(ctx, c.String("metrics-port"), true, metrics.AtlasRegistry)

	conf := config.NewControllerConfig()
	conf.ADSAddress = c.String("envoy-ads-address")
	conf.ADSPort = c.Int64("envoy-ads-port")
	conf.EnvoyAddress = c.String("envoy-address")

	cfg, err := kubeconfig.GetNonInteractiveClientConfig(c.String("kubeconfig")).ClientConfig()
	if err != nil {
		return err
	}

	apply, err := apply.NewForConfig(cfg)
	if err != nil {
		return err
	}

	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	core, err := core.NewFactoryFromConfig(cfg)
	if err != nil {
		return err
	}

	atlas, err := atlas.Register(ctx, conf, log, c, apply, core.Core().V1().Secret(), core.Core().V1().ConfigMap(), core.Core().V1().Service())
	if err != nil {
		return err
	}

	leader.RunOrDie(ctx, c.String("namespace"), c.String("lockname"), kube, func(ctx context.Context) {
		runtime.Must(atlas.Setup())
		runtime.Must(start.All(ctx, 50, core))

		<-ctx.Done()
	})

	return nil
}

func init() {
	cmd := controlCommand{}

	flags := []cli.Flag{
		&cli.StringFlag{
			Name:    "metrics-port",
			Usage:   "Port for the metrics and debug http server to listen on",
			EnvVars: []string{"METRICS_PORT", "CONTROLLER_METRICS_PORT"},
			Value:   "6309",
		},
		&cli.StringFlag{
			Name:    "namespace",
			Usage:   "namespace to use for leader election",
			EnvVars: []string{"NAMESPACE"},
			Value:   common.MonitoringNamespace,
		},
		&cli.StringFlag{
			Name:    "lockname",
			Usage:   "name of the lock for leader election",
			EnvVars: []string{"LOCKNAME"},
			Value:   common.NAME,
		},
		&cli.StringFlag{
			Name:    "envoy-address",
			Usage:   "FQDN or IP of Atlas' Envoy Server",
			EnvVars: []string{"ATLAS_ENVOY_ADDRESS"},
			Value:   "localhost",
		},
		&cli.StringFlag{
			Name:    "envoy-ads-address",
			Usage:   "FQDN or IP of Atlas' Aggreggated Discovery Service (ADS) Server",
			EnvVars: []string{"ATLAS_ENVOY_ADS_ADDRESS"},
			Value:   "localhost",
		},
		&cli.Int64Flag{
			Name:    "envoy-ads-port",
			Usage:   "The port for Atlas' Aggreggated Discovery Service (ADS) Server",
			EnvVars: []string{"ATLAS_ENVOY_ADS_PORT"},
			Value:   10900,
		},
		&cli.StringFlag{
			Name:    "dns-config-map-name",
			Usage:   "The name of the ConfigMap used for CoreDNS config and zone data",
			EnvVars: []string{"ATLAS_DNS_CM_NAME"},
			Value:   common.DNSConfigMapName,
		},
	}

	cliCmd := &cli.Command{
		Name:   "controller",
		Usage:  "Run Atlas Controllers",
		Action: cmd.Execute,
		Flags:  append(flags, globalFlags()...),
		Before: globalBefore,
	}

	common.RegisterCommand(cliCmd)
}
