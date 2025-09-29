package mqttx

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// MetricsExporter 指标导出接口
type MetricsExporter interface {
	Export(metrics map[string]interface{}) error
}

// PrometheusExporter Prometheus 格式导出器
type PrometheusExporter struct {
	prefix string
}

func NewPrometheusExporter(prefix string) *PrometheusExporter {
	return &PrometheusExporter{prefix: prefix}
}

func (e *PrometheusExporter) Export(metrics map[string]interface{}) string {
	var sb strings.Builder
	timestamp := time.Now().Unix() * 1000

	for k, v := range metrics {
		metricName := fmt.Sprintf("%s_%s", e.prefix, strings.Replace(k, "-", "_", -1))
		switch val := v.(type) {
		case float64:
			sb.WriteString(fmt.Sprintf("%s %f %d\n", metricName, val, timestamp))
		case uint64:
			sb.WriteString(fmt.Sprintf("%s %d %d\n", metricName, val, timestamp))
		case int64:
			sb.WriteString(fmt.Sprintf("%s %d %d\n", metricName, val, timestamp))
		case int:
			sb.WriteString(fmt.Sprintf("%s %d %d\n", metricName, val, timestamp))
		case string:
			// 时间格式，忽略
		}
	}

	return sb.String()
}

// JSONExporter JSON 格式导出器
type JSONExporter struct {
	pretty bool
}

func NewJSONExporter(pretty bool) *JSONExporter {
	return &JSONExporter{pretty: pretty}
}

func (e *JSONExporter) Export(metrics map[string]interface{}) (string, error) {
	if e.pretty {
		data, err := json.MarshalIndent(metrics, "", "  ")
		return string(data), err
	}
	data, err := json.Marshal(metrics)
	return string(data), err
}
