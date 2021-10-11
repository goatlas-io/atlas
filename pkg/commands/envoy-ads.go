package commands

import (
	"context"
	"fmt"

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
	"github.com/ekristen/atlas/pkg/envoy"
	"github.com/ekristen/atlas/pkg/metrics"
)

type envoyADSCommand struct{}

func (s *envoyADSCommand) Execute(c *cli.Context) error {
	// set up signals so we handle the first shutdown signal gracefully
	ctx := signals.SetupSignalHandler(context.Background())

	log := logrus.WithField("command", "envoy-ads")

	go metrics.NewMetricsServer(ctx, c.String("metrics-port"), true, metrics.EnvoyAdsRegistry)

	conf := config.NewEnvoyADSConfig()
	conf.AtlasEnvoyAddress = c.String("envoy-address")

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

	envoyads := envoy.Register(ctx, conf, log, apply, c,
		core.Core().V1().Service(),
		core.Core().V1().Secret())

	// Become leader, then create CRDS (or update), followed by starting all controllers
	leader.RunOrDie(ctx, c.String("namespace"), c.String("lockname"), kube, func(ctx context.Context) {
		runtime.Must(start.All(ctx, 50, core))
		runtime.Must(envoyads.Start(ctx, c.Int("grpc-port"), c.Int64("node-id"), c.Bool("debug-envoy")))

		<-ctx.Done()
	})

	return nil
}

func init() {
	cmd := envoyADSCommand{}

	flags := []cli.Flag{
		&cli.Int64Flag{
			Name:    "node-id",
			Usage:   "Node ID (must be 1-1024)",
			EnvVars: []string{"NODE_ID", "ENVOY_ADS_NODE_ID"},
			Value:   1,
		},
		&cli.IntFlag{
			Name:    "grpc-port",
			Usage:   "Port for the GPRC Interface of the Management Server",
			EnvVars: []string{"GRPC_PORT", "ENVOY_ADS_GRPC_PORT", "ATLAS_ENVOY_ADS_GRPC_PORT"},
			Value:   6305,
		},
		&cli.IntFlag{
			Name:    "metrics-port",
			Usage:   "Port for the metrics and debug http server to listen on",
			EnvVars: []string{"METRICS_PORT", "ENVOY_ADS_METRICS_PORT"},
			Value:   6309,
		},
		&cli.StringFlag{
			Name:    "envoy-address",
			Usage:   "FQDN or IP of Atlas' Envoy Server",
			EnvVars: []string{"ATLAS_ENVOY_ADDRESS"},
			Value:   "localhost",
		},
		&cli.StringFlag{
			Name:    "alertmanager-selector",
			Usage:   "Label Selector for AlertManager",
			EnvVars: []string{"ATLAS_ALERTMANAGER_SELECTOR"},
			Value:   common.ObservabilityAlertManagerServiceLabel,
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
			Value:   fmt.Sprintf("%s-envoy-ads", common.NAME),
		},
		&cli.BoolFlag{
			Name:   "debug-envoy",
			Usage:  "used to add extra static resources to envoy ads snapshots",
			Hidden: true,
		},
	}

	cliCmd := &cli.Command{
		Name:   "envoy-ads",
		Usage:  "Run Envoy Aggregated Discovery Service (ADS)",
		Action: cmd.Execute,
		Flags:  append(flags, globalFlags()...),
		Before: globalBefore,
	}

	common.RegisterCommand(cliCmd)
}
