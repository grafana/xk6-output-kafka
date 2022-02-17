package kafka

import (
	"github.com/knmsk/xk6-output-kafka/pkg/kafka"
	"go.k6.io/k6/output"
)

func init() {
	output.RegisterExtension("xk6-kafka", func(p output.Params) (output.Output, error) {
		return kafka.New(p)
	})
}
