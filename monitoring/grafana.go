package monitoring

import (
	"bytes"
	"context"
	_ "embed"
	"github.com/skip-mev/petri/provider"
	"text/template"
)

const DEFAULT_PROMETHEUS_URL = "http://prometheus:9090"

type GrafanaOptions struct {
	PrometheusURL string
}

//go:embed files/grafana/config/config.ini
var grafanaConfig string

//go:embed files/grafana/config/datasources.yml
var grafanaDatasourceTemplate string

//go:embed files/grafana/config/dashboards.yml
var grafanaDashboardProvisioningConfig string

//go:embed files/grafana/config/dashboard.json
var grafanaDashboardJSON string

func SetupGrafanaTask(ctx context.Context, p provider.Provider, opts GrafanaOptions) (*provider.Task, error) {
	task, err := provider.CreateTask(ctx, p, provider.TaskDefinition{
		Name: "grafana",
		Image: provider.ImageDefinition{
			Image: "grafana/grafana:latest",
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
	})

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

	err = task.WriteFile(ctx, "conf/provisioning/dashboards/dashboard.json", []byte(grafanaDashboardJSON))

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
