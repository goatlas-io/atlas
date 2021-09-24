package commands

import (
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func globalFlags() []cli.Flag {
	globalFlags := []cli.Flag{
		&cli.StringFlag{
			Name:    "kubeconfig",
			Usage:   "Kube config for accessing k8s cluster",
			EnvVars: []string{"KUBECONFIG"},
		},
		&cli.StringFlag{
			Name:    "log-level",
			Usage:   "Log Level",
			Aliases: []string{"l"},
			EnvVars: []string{"LOGLEVEL"},
			Value:   "info",
		},
		&cli.StringFlag{
			Name:  "config",
			Usage: "configuration file",
		},
	}

	return globalFlags
}

func globalBefore(c *cli.Context) error {
	switch c.String("log-level") {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	}

	return nil
}
