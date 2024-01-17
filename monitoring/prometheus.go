package monitoring

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"github.com/skip-mev/petri/provider/v2"
	"go.uber.org/zap"
	"strings"
	"text/template"
)

//go:embed files/prometheus/config/prometheus.yml
var prometheusConfigTemplate string

type PrometheusOptions struct {
	Targets []string
}

func SetupPrometheusTask(ctx context.Context, logger *zap.Logger, p provider.Provider, opts PrometheusOptions) (*provider.Task, error) {
	task, err := provider.CreateTask(ctx, logger, p, provider.TaskDefinition{
		Name: "prometheus",
		Image: provider.ImageDefinition{
			Image: "prom/prometheus:latest",
			UID:   "65534",
			GID:   "65534",
		},
		Ports: []string{
			"3000",
		},
		DataDir: "/prometheus_config",
		Entrypoint: []string{
			"/bin/prometheus",
			"--config.file=/prometheus_config/prometheus.yml",
			"--storage.tsdb.path=/prometheus",
			"--web.console.libraries=/usr/share/prometheus/console_libraries",
			"--web.console.templates=/usr/share/prometheus/consoles",
		},
	})

	parsedPrometheusConfig, err := parsePrometheusConfig(opts)
	if err != nil {
		return nil, err
	}

	err = task.WriteFile(ctx, "prometheus.yml", []byte(parsedPrometheusConfig))

	if err != nil {
		return nil, err
	}

	return task, nil
}

func parsePrometheusConfig(opts PrometheusOptions) (string, error) {
	parsedPrometheusConfig, err := template.New("prometheus.yml").Parse(prometheusConfigTemplate)

	if err != nil {
		return "", err
	}

	var buf bytes.Buffer

	variables := struct {
		Targets string
	}{
		Targets: listToString(opts.Targets),
	}

	err = parsedPrometheusConfig.Execute(&buf, variables)

	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func listToString(list []string) string {
	var result strings.Builder

	result.WriteString("[")
	for i, str := range list {
		result.WriteString(fmt.Sprintf("\"%s\"", str))
		if i < len(list)-1 {
			result.WriteString(", ")
		}
	}

	result.WriteString("]")

	return result.String()
}
