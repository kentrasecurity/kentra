package main

import (
	"flag"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// Config holds all command-line flags
type Config struct {
	metricsAddr          string
	metricsCertPath      string
	metricsCertName      string
	metricsCertKey       string
	webhookCertPath      string
	webhookCertName      string
	webhookCertKey       string
	probeAddr            string
	enableLeaderElection bool
	secureMetrics        bool
	enableHTTP2          bool
	zapOpts              zap.Options
}

func parseFlags() *Config {
	cfg := &Config{
		zapOpts: zap.Options{Development: true},
	}

	flag.StringVar(&cfg.metricsAddr, "metrics-bind-address", "0",
		"The address the metrics endpoint binds to. "+
			"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&cfg.probeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	flag.BoolVar(&cfg.enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&cfg.secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&cfg.webhookCertPath, "webhook-cert-path", "",
		"The directory that contains the webhook certificate.")
	flag.StringVar(&cfg.webhookCertName, "webhook-cert-name", "tls.crt",
		"The name of the webhook certificate file.")
	flag.StringVar(&cfg.webhookCertKey, "webhook-cert-key", "tls.key",
		"The name of the webhook key file.")
	flag.StringVar(&cfg.metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&cfg.metricsCertName, "metrics-cert-name", "tls.crt",
		"The name of the metrics server certificate file.")
	flag.StringVar(&cfg.metricsCertKey, "metrics-cert-key", "tls.key",
		"The name of the metrics server key file.")
	flag.BoolVar(&cfg.enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")

	cfg.zapOpts.BindFlags(flag.CommandLine)
	flag.Parse()

	return cfg
}
