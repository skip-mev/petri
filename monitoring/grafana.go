package monitoring

import (
	"bytes"
	"context"
	_ "embed"
	"github.com/skip-mev/petri/provider/v2"
	"go.uber.org/zap"
	"text/template"
)

const DEFAULT_PROMETHEUS_URL = "http://prometheus:9090"

type GrafanaOptions struct {
	PrometheusURL          string      // The URL of the Prometheus instance. This needs to be accessible from the Grafana container.
	DashboardJSON          string      // The JSON of the Grafana dashboard to be provisioned. You can get the JSON by exporting a dashboard in the Grafana web interface
	ProviderSpecificConfig interface{} // Provider-specific configuration for the Grafana task
}

//go:embed files/grafana/config/config.ini
var grafanaConfig string

//go:embed files/grafana/config/datasources.yml
var grafanaDatasourceTemplate string

//go:embed files/grafana/config/dashboards.yml
var grafanaDashboardProvisioningConfig string

// SetupGrafanaTask sets up and configures (but does not start) a Grafana task.
// Additionally, it creates a Prometheus datasource and a dashboard (given the DashboardJSON in GrafanaOptions).
func SetupGrafanaTask(ctx context.Context, logger *zap.Logger, p provider.Provider, opts GrafanaOptions) (*provider.Task, error) {
	task, err := provider.CreateTask(ctx, logger, p, provider.TaskDefinition{
		Name: "grafana",
		Image: provider.ImageDefinition{
			Image: "grafana/grafana:main",
			UID:   "472",
			GID:   "0",
		},
		Ports: []string{
			"3000",
		},
		DataDir: "/grafana",
		Environment: map[string]string{
			"GF_PATHS_PROVISIONING": "/grafana/conf/provisioning",
		},
		Entrypoint: []string{
			"grafana",
			"server",
			"--homepath=/usr/share/grafana",
			"--config=/grafana/grafana.ini",
			"--packaging=docker",
			"cfg:default.log.mode=console",
			"cfg:default.paths.data=/var/lib/grafana",
			"cfg:default.paths.logs=/var/log/grafana",
			"cfg:default.paths.plugins=/var/lib/grafana/plugins",
			"cfg:default.paths.provisioning=/grafana/conf/provisioning",
		},
		ProviderSpecificConfig: opts.ProviderSpecificConfig,
	})

	if err != nil {
		return nil, err
	}

	err = task.WriteFile(ctx, "grafana.ini", []byte(grafanaConfig))

	if err != nil {
		return nil, err
	}

	parsedGrafanaDatasourceConfig, err := parseGrafanaDatasourceTemplate(opts)
	if err != nil {
		return nil, err
	}

	err = task.WriteFile(ctx, "conf/provisioning/datasources/prometheus.yml", []byte(parsedGrafanaDatasourceConfig))

	if err != nil {
		return nil, err
	}

	err = task.WriteFile(ctx, "conf/provisioning/dashboards/dashboards.yml", []byte(grafanaDashboardProvisioningConfig))

	if err != nil {
		return nil, err
	}

	err = task.WriteFile(ctx, "conf/provisioning/dashboards/dashboard.json", []byte(opts.DashboardJSON))

	if err != nil {
		return nil, err
	}

	return task, nil
}

func parseGrafanaDatasourceTemplate(opts GrafanaOptions) (string, error) {
	parsedGrafanaDatasourceTemplate, err := template.New("datasources.yml").Parse(grafanaDatasourceTemplate)

	if err != nil {
		return "", err
	}

	var buf bytes.Buffer

	variables := struct {
		URL string
	}{
		URL: opts.PrometheusURL,
	}

	if variables.URL == "" {
		variables.URL = DEFAULT_PROMETHEUS_URL
	}

	err = parsedGrafanaDatasourceTemplate.Execute(&buf, variables)

	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
