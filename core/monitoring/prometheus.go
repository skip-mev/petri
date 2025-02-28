package monitoring

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	"go.uber.org/zap"

	"github.com/skip-mev/petri/core/v3/provider"
)

//go:embed files/prometheus/config/prometheus.yml
var prometheusConfigTemplate string

type PrometheusOptions struct {
	Targets                []string
	ProviderSpecificConfig map[string]string
}

// SetupPrometheusTask sets up and configures (but does not start) a Prometheus task.
// Additionally, it creates a Prometheus configuration file (given the Targets in PrometheusOptions).
func SetupPrometheusTask(ctx context.Context, logger *zap.Logger, p provider.ProviderI, opts PrometheusOptions) (provider.TaskI, error) {
	task, err := p.CreateTask(ctx, provider.TaskDefinition{
		Name:          "prometheus",
		ContainerName: "prometheus",
		Image: provider.ImageDefinition{
			Image: "prom/prometheus:v2.46.0",
			UID:   "65534",
			GID:   "65534",
		},
		Ports: []string{
			"3000",
		},
		DataDir: "/prometheus",
		Entrypoint: []string{
			"/bin/prometheus",
			"--config.file=/prometheus/prometheus.yml",
			"--storage.tsdb.path=/prometheus",
			"--web.console.libraries=/usr/share/prometheus/console_libraries",
			"--web.console.templates=/usr/share/prometheus/consoles",
		},
		ProviderSpecificConfig: opts.ProviderSpecificConfig,
	})
	if err != nil {
		return nil, err
	}

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
