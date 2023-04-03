// Package kafka registers the xk6-output-kafka extension
package kafka

import (
	"github.com/grafana/xk6-output-kafka/pkg/kafka"
	"go.k6.io/k6/output"
)

func init() {
	output.RegisterExtension("xk6-kafka", kafka.New)
}
