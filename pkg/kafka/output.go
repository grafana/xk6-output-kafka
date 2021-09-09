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

package kafka

import (
	"crypto/tls"
	"encoding/json"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/output"
	"go.k6.io/k6/stats"
)

const flushPeriod = 1 * time.Second

type Output struct {
	output.SampleBuffer

	Params          output.Params
	periodicFlusher *output.PeriodicFlusher

	Config   Config
	CloseFn  func() error
	logger   logrus.FieldLogger
	Producer sarama.AsyncProducer
}

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
			saramaConfig.Net.TLS.Config = &tls.Config{
				InsecureSkipVerify: config.Insecure.Bool,
				ClientAuth:         0,
			}
		}
		switch saramaAuthMechanism {
		case "plain":
			saramaConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		case "scram-sha-512":
			saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
			saramaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &xDGSCRAMClient{HashGeneratorFcn: SHA512} }
		case "scram-sha-256":
			saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
			saramaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &xDGSCRAMClient{HashGeneratorFcn: SHA256} }
		}
	}

	version, err := sarama.ParseKafkaVersion(config.Version.String)

	if err != nil {
		return nil, err
	}

	saramaConfig.Version = version

	return sarama.NewAsyncProducer(config.Brokers, saramaConfig)
}

func (o *Output) Description() string {
	return "xk6-kafka"
}

func (o *Output) Start() error {
	// TODO get the period on the config
	periodicFlusher, err := output.NewPeriodicFlusher(flushPeriod, o.flushMetrics)
	if err != nil {
		return err
	}
	o.periodicFlusher = periodicFlusher

	return nil
}

func (o *Output) Stop() error {
	o.logger.Debug("Kafka: Stopping...")
	defer o.logger.Debug("Kafka: Stopped!")
	o.periodicFlusher.Stop()
	o.Producer.AsyncClose()

	if o.Config.LogError.Bool {
		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			for err := range o.Producer.Errors() {
				o.logger.WithError(err.Err).Error("Kafka: failed to send message.")
			}
			wg.Done()
		}()
		wg.Wait()
	}

	return nil
}

func (o *Output) batchFromBufferedSamples(bufferedSamples []stats.SampleContainer) ([]string, error) {

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

func (o *Output) formatSamples(samples stats.Samples) ([]string, error) {
	var metrics []string

	switch o.Config.Format.String {
	case "influxdb":
		var err error
		fieldKinds, err := makeInfluxdbFieldKinds(o.Config.InfluxDBConfig.TagsAsFields)
		if err != nil {
			return nil, err
		}
		metrics, err = formatAsInfluxdbV1(o.logger, samples, newExtractTagsFields(fieldKinds))
		if err != nil {
			return nil, err
		}
	default:
		for _, sample := range samples {
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
