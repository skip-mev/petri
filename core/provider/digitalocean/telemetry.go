package digitalocean

import (
	"encoding/json"
	"fmt"
)

type PrometheusSettings struct {
	Username string `json:"username"`
	Password string `json:"password"`
	URL      string `json:"url"`
}

type LokiSettings struct {
	Username string `json:"username"`
	Password string `json:"password"`
	URL      string `json:"url"`
}

type TelemetrySettings struct {
	Prometheus PrometheusSettings `json:"prometheus"`
	Loki       LokiSettings       `json:"loki"`
}

type TelemetryConfig struct {
	Prometheus PrometheusSettings `json:"prometheus"`
	Loki       LokiSettings       `json:"loki"`
	Provider   string             `json:"provider"`
}

func (t *TelemetrySettings) GetCommand(provider string) ([]string, error) {
	telemetryConfig := TelemetryConfig{
		Prometheus: t.Prometheus,
		Loki:       t.Loki,
		Provider:   provider,
	}

	config, err := json.Marshal(telemetryConfig)

	if err != nil {
		return nil, err
	}

	command := fmt.Sprintf("echo '%s' > /etc/alloy/telemetry.json", string(config))
	return []string{
		command,
		"systemctl restart alloy",
	}, nil
}
