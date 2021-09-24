package commands

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/signals"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/ekristen/atlas/pkg/common"
)

//go:embed templates/*
var templates embed.FS

type clusterValuesCommand struct {
}

func (w *clusterValuesCommand) Execute(c *cli.Context) error {
	format := c.String("format")
	if format != "raw" && format != "helm-chart" && format != "helm-release" {
		return fmt.Errorf("Invalid format provided, valid options are: raw, helm-chart, helm-release")
	}

	// set up signals so we handle the first shutdown signal gracefully
	ctx := signals.SetupSignalHandler(context.Background())

	log := logrus.WithField("command", "cluster-add").WithField("cluster", c.String("name"))

	cfg, err := kubeconfig.GetNonInteractiveClientConfig(c.String("kubeconfig")).ClientConfig()
	if err != nil {
		return err
	}

	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	secretName := "atlas-envoy-values"
	if c.String("name") != "atlas" {
		secretName = fmt.Sprintf("%s-envoy-values", c.String("name"))
	}

	secret, err := kube.CoreV1().Secrets(c.String("namespace")).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	log.Debug(secret)

	data := struct {
		Namespace string
		Values    string
	}{
		Namespace: c.String("namespace"),
		Values:    string(secret.Data["values.yaml"]),
	}

	d, err := templates.ReadFile(fmt.Sprintf("templates/%s.tmpl", c.String("format")))
	if err != nil {
		logrus.WithError(err).Error("unable to read in template")
		return err
	}

	tmpl, err := template.New("zone").Funcs(sprig.TxtFuncMap()).Parse(string(d))
	if err != nil {
		logrus.WithError(err).Error("unable to parse template")
		return err
	}

	var buf bytes.Buffer

	if err := tmpl.Execute(&buf, data); err != nil {
		logrus.WithError(err).Error("unable to execute template")
		return err
	}

	fmt.Println(string(buf.Bytes()))

	return nil
}

func init() {
	cmd := clusterValuesCommand{}

	flags := []cli.Flag{
		&cli.StringFlag{
			Name:     "name",
			Usage:    "Name of the cluster, must be DNS compliant",
			Required: true,
		},
		&cli.StringFlag{
			Name:  "namespace",
			Usage: "namespace where atlas resources are located",
			Value: "monitoring",
		},
		&cli.StringFlag{
			Name:  "format",
			Usage: "Format of the values output (raw, helm-chart, helm-release)",
			Value: "raw",
		},
	}

	cliCmd := &cli.Command{
		Name:   "cluster-values",
		Usage:  "get cluster envoy values for helm chart",
		Flags:  append(flags, globalFlags()...),
		Before: globalBefore,
		Action: cmd.Execute,
	}

	common.RegisterCommand(cliCmd)
}
