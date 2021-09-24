package commands

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/signals"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

	"github.com/ekristen/atlas/pkg/common"
)

type clusterAddCommand struct {
}

func (w *clusterAddCommand) Execute(c *cli.Context) error {
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

	service, err := kube.CoreV1().Services(c.String("namespace")).Get(ctx, c.String("name"), metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	notfound := false
	if apierrors.IsNotFound(err) {
		notfound = true
	}

	log.Debug(service)

	if notfound || c.Bool("overwrite") {
		newService := service.DeepCopy()
		newService.ObjectMeta.Labels = map[string]string{
			common.ThanosClusterLabel: "true",
			common.ReplicasLabel:      c.String("replicas"),
		}
		newService.Spec = corev1.ServiceSpec{
			ClusterIP:  "None",
			ClusterIPs: []string{},
			Ports: []corev1.ServicePort{
				{
					Port: 9090,
					TargetPort: intstr.IntOrString{
						IntVal: 9090,
					},
					Protocol: "TCP",
					Name:     "prometheus",
				},
				{
					Port: 10901,
					TargetPort: intstr.IntOrString{
						IntVal: 10901,
					},
					Protocol: "TCP",
					Name:     "thanos",
				},
				{
					Port: 9093,
					TargetPort: intstr.IntOrString{
						IntVal: 9093,
					},
					Protocol: "TCP",
					Name:     "alertmanager",
				},
			},
			ExternalIPs: c.StringSlice("external-ip"),
		}

		/*
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      c.String("name"),
					Namespace: c.String("namespace"),
					Labels: map[string]string{
						common.ReplicasLabel: c.String("replicas"),
					},
				},
				Spec: corev1.ServiceSpec{
					ClusterIP:  "None",
					ClusterIPs: []string{},
					Ports: []corev1.ServicePort{
						{
							Port: 9090,
							TargetPort: intstr.IntOrString{
								IntVal: 9090,
							},
							Protocol: "TCP",
							Name:     "prometheus",
						},
						{
							Port: 10901,
							TargetPort: intstr.IntOrString{
								IntVal: 10901,
							},
							Protocol: "TCP",
							Name:     "thanos",
						},
						{
							Port: 9093,
							TargetPort: intstr.IntOrString{
								IntVal: 9093,
							},
							Protocol: "TCP",
							Name:     "alertmanager",
						},
					},
					ExternalIPs: []string{
						c.String("external-ip"),
					},
				},
			}
		*/

		if notfound {
			if _, err := kube.CoreV1().Services(c.String("namespace")).Create(ctx, newService, metav1.CreateOptions{}); err != nil {
				return err
			}
			log.Info("Cluster added successfully")
		} else {
			if c.Bool("overwrite") {
				if _, err := kube.CoreV1().Services(c.String("namespace")).Update(ctx, newService, metav1.UpdateOptions{}); err != nil {
					return err
				}
				log.Info("Cluster updated successfully")
			}
		}
	} else {
		log.Warn("Cluster already exists, use --overwrite to apply specified values")
	}

	return nil
}

func init() {
	cmd := clusterAddCommand{}

	flags := []cli.Flag{
		&cli.StringFlag{
			Name:     "name",
			Usage:    "Name of the cluster, must be DNS compliant",
			Required: true,
		},
		&cli.IntFlag{
			Name:  "replicas",
			Usage: "The number of prometheus/thanos replicas",
			Value: 1,
		},
		&cli.StringFlag{
			Name:  "namespace",
			Usage: "namespace where atlas resources are located",
			Value: "monitoring",
		},
		&cli.StringSliceFlag{
			Name:     "external-ip",
			Usage:    "downstream cluster IP adddress",
			Required: true,
		},
		&cli.BoolFlag{
			Name:  "overwrite",
			Usage: "Overwrite values if cluster already exists",
		},
	}

	cliCmd := &cli.Command{
		Name:   "cluster-add",
		Usage:  "add (or update) cluster to atlas",
		Flags:  append(flags, globalFlags()...),
		Before: globalBefore,
		Action: cmd.Execute,
	}

	common.RegisterCommand(cliCmd)
}
