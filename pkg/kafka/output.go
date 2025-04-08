/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

// Package kafka sends real-time testing metrics to an Apache Kafka message broker
package kafka

import (
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
)

const flushPeriod = 1 * time.Second

// Output is a k6 output that sends metrics to a Kafka broker.
type Output struct {
	output.SampleBuffer

	periodicFlusher *output.PeriodicFlusher

	Config   Config
	CloseFn  func() error
	logger   logrus.FieldLogger
	Producer sarama.AsyncProducer
	errorsWg sync.WaitGroup
}

// New creates a new instance of the output.
func New(params output.Params) (output.Output, error) {
	return newOutput(params)
}

func newOutput(params output.Params) (*Output, error) {
	config, err := GetConsolidatedConfig(params.JSONConfig, params.Environment, params.ConfigArgument)
	if err != nil {
		return nil, err
	}

	producer, err := newProducer(config)
	if err != nil {
		return nil, err
	}

	return &Output{
		Producer: producer,
		logger:   params.Logger,
		Config:   config,
	}, nil
}

func newProducer(config Config) (sarama.AsyncProducer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.Return.Errors = config.LogError.Bool

	saramaAuthMechanism := config.AuthMechanism.String

	if saramaAuthMechanism != "none" {
		saramaConfig.Net.SASL.Enable = true
		saramaConfig.Net.SASL.Handshake = true
		saramaConfig.Net.SASL.User = config.User.String
		saramaConfig.Net.SASL.Password = config.Password.String
		if config.SSL.Bool {
			saramaConfig.Net.TLS.Enable = true
			// #nosec G402
			saramaConfig.Net.TLS.Config = &tls.Config{
				InsecureSkipVerify: config.InsecureSkipTLSVerify.Bool,
				ClientAuth:         0,
			}
		}
		switch saramaAuthMechanism {
		case "plain":
			saramaConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		case "scram-sha-512":
			saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
			saramaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
				return &xDGSCRAMClient{HashGeneratorFcn: sha512.New}
			}
		case "scram-sha-256":
			saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
			saramaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
				return &xDGSCRAMClient{HashGeneratorFcn: sha256.New}
			}
		}
	}

	version, err := sarama.ParseKafkaVersion(config.Version.String)
	if err != nil {
		return nil, err
	}

	saramaConfig.Version = version

	return sarama.NewAsyncProducer(config.Brokers, saramaConfig)
}

// Description returns a short human-readable description of the output.
func (o *Output) Description() string {
	return fmt.Sprintf("xk6-Kafka: Kafka Async output on topic %v", o.Config.Topic.String)
}

// Start initializes the output.
func (o *Output) Start() error {
	// TODO get the period on the config
	periodicFlusher, err := output.NewPeriodicFlusher(flushPeriod, o.flushMetrics)
	if err != nil {
		return err
	}
	o.periodicFlusher = periodicFlusher

	if o.Config.LogError.Bool {
		o.errorsWg.Add(1)
		go func() {
			// Errors is the error output channel back to the user. You MUST read from this
			// channel or the Producer will deadlock when the channel is full.
			// reference: https://pkg.go.dev/github.com/shopify/sarama#AsyncProducer
			for err := range o.Producer.Errors() {
				o.logger.WithError(err.Err).Error("Kafka: failed to send message.")
			}
			o.errorsWg.Done()
		}()
	}
	return nil
}

// Stop stops the output.
func (o *Output) Stop() error {
	o.logger.Debug("Kafka: Stopping...")
	defer o.logger.Debug("Kafka: Stopped!")
	o.periodicFlusher.Stop()
	o.Producer.AsyncClose()
	o.errorsWg.Wait()

	return nil
}

func (o *Output) batchFromBufferedSamples(bufferedSamples []metrics.SampleContainer) ([]string, error) {
	var formattedSamples []string
	for _, bufferedSample := range bufferedSamples {
		samples := bufferedSample.GetSamples()
		formatSamples, err := o.formatSamples(samples)
		if err != nil {
			return nil, err
		}
		formattedSamples = append(formattedSamples, formatSamples...)
	}
	return formattedSamples, nil
}

var allowedMetrics = map[string]struct{} {
    "http_reqs":         {},
    "http_req_duration": {},
    "data_sent":         {},
    "data_received":     {},
    "vus":               {},
}

var allowedMetricsCount = len(allowedMetrics) // presumably one metric of each type in a samples batch

func isAllowedMetric(metricName string) bool {
    _, exists := allowedMetrics[metricName]
    return exists
}

func filterSamplesByMetricNames(samples metrics.Samples) metric.Samples {
    filteredSamples := make(metrics.Samples, 0, allowedMetricsCount)
    for _, sample := range samples {
        if isAllowedMetric(sample.Metric.Name) {
            filteredSamples = append(filteredSamples, sample)
        }
    }
    return filteredSamples
}

func (o *Output) formatSamples(samples metrics.Samples) ([]string, error) {
	var metrics []string

	filteredSamples := filterSamplesByMetricNames(samples)

	switch o.Config.Format.String {
	case "influxdb":
		var err error
		fieldKinds, err := makeInfluxdbFieldKinds(o.Config.InfluxDBConfig.TagsAsFields)
		if err != nil {
			return nil, err
		}
		metrics, err = formatAsInfluxdbV1(o.logger, filteredSamples, newExtractTagsFields(fieldKinds))
		if err != nil {
			return nil, err
		}
	default:
		for _, sample := range filteredSamples {
			metric, err := json.Marshal(wrapSample(sample))
			if err != nil {
				return nil, err
			}

			metrics = append(metrics, string(metric))
		}
	}

	return metrics, nil
}

func (o *Output) flushMetrics() {
	bufferedSamples := o.GetBufferedSamples()

	o.logger.Debug("Kafka: Converting the samples to messages...")
	messages, err := o.batchFromBufferedSamples(bufferedSamples)
	if err != nil {
		o.logger.WithError(err).Error("Kafka: Error getting the messages")
	}

	startTime := time.Now()
	o.logger.Debug("Kafka: Delivering...")
	for _, message := range messages {
		o.Producer.Input() <- &sarama.ProducerMessage{Topic: o.Config.Topic.String, Value: sarama.StringEncoder(message)}
	}
	t := time.Since(startTime)
	o.logger.WithField("t", t).Debug("Kafka: Delivered!")
}
