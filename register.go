package kafka

import (
	"github.com/k6io/xk6-output-kafka/pkg/kafka"
	"github.com/loadimpact/k6/output"
)

func init() {
	output.RegisterExtension("xk6-kafka", func(p output.Params) (output.Output, error) {
		return kafka.New(p)
	})
}
