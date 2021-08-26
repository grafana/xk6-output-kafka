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
	"encoding/json"
	"errors"
	"time"

	"github.com/Shopify/sarama"
	"github.com/kelseyhightower/envconfig"
	"github.com/kubernetes/helm/pkg/strvals"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib/types"
)

// Config is the config for the kafka collector
type Config struct {
	// Connection.
	Brokers []string `json:"brokers" envconfig:"K6_KAFKA_BROKERS"`

	// Samples.
	Topic         null.String        `json:"topic" envconfig:"K6_KAFKA_TOPIC"`
	User          null.String        `json:"user" envconfig:"K6_KAFKA_SASL_USER"`
	Password      null.String        `json:"password" envconfig:"K6_KAFKA_SASL_PASSWORD"`
	AuthMechanism null.String        `json:"auth_mechanism" envconfig:"K6_KAFKA_AUTH_MECHANISM"`
	Format        null.String        `json:"format" envconfig:"K6_KAFKA_FORMAT"`
	PushInterval  types.NullDuration `json:"push_interval" envconfig:"K6_KAFKA_PUSH_INTERVAL"`
	Version       null.String        `json:"version" envconfig:"K6_KAFKA_VERSION"`
	SSL           bool               `json:"ssl" envconfig:"K6_KAFKA_SSL"`
	Insecure      bool               `json:"insecure" envconfig:"K6_KAFKA_INSECURE"`

	InfluxDBConfig influxdbConfig `json:"influxdb"`
}

// config is a duplicate of ConfigFields as we can not mapstructure.Decode into
// null types so we duplicate the struct with primitive types to Decode into
type config struct {
	Brokers       []string `json:"brokers" mapstructure:"brokers" envconfig:"K6_KAFKA_BROKERS"`
	Topic         string   `json:"topic" mapstructure:"topic" envconfig:"K6_KAFKA_TOPIC"`
	Format        string   `json:"format" mapstructure:"format" envconfig:"K6_KAFKA_FORMAT"`
	PushInterval  string   `json:"push_interval" mapstructure:"push_interval" envconfig:"K6_KAFKA_PUSH_INTERVAL"`
	User          string   `json:"user" mapstructure:"user" envconfig:"K6_KAFKA_SASL_USER"`
	Password      string   `json:"password" mapstructure:"password" envconfig:"K6_KAFKA_SASL_PASSWORD"`
	AuthMechanism string   `json:"auth_mechanism" mapstructure:"auth_mechanism" envconfig:"K6_KAFKA_AUTH_MECHANISM"`

	InfluxDBConfig influxdbConfig `json:"influxdb" mapstructure:"influxdb"`
	Version        string         `json:"version" mapstructure:"version"`
	SSL            bool           `json:"ssl" mapstructure:"ssl"`
	Insecure       bool           `json:"insecure" mapstructure:"insecure"`
}

// NewConfig creates a new Config instance with default values for some fields.
func NewConfig() Config {
	return Config{
		Format:         null.StringFrom("json"),
		PushInterval:   types.NullDurationFrom(1 * time.Second),
		InfluxDBConfig: newInfluxdbConfig(),
		AuthMechanism:  null.StringFrom("none"),
		Version:        null.StringFrom(sarama.DefaultVersion.String()),
		SSL:            false,
		Insecure:       false,
	}
}

func (c Config) Apply(cfg Config) Config {
	if len(cfg.Brokers) > 0 {
		c.Brokers = cfg.Brokers
	}
	if cfg.Format.Valid {
		c.Format = cfg.Format
	}
	if cfg.Topic.Valid {
		c.Topic = cfg.Topic
	}
	if cfg.PushInterval.Valid {
		c.PushInterval = cfg.PushInterval
	}
	if cfg.AuthMechanism.Valid {
		c.AuthMechanism = cfg.AuthMechanism
	}
	if cfg.User.Valid {
		c.User = cfg.User
	}
	if cfg.Password.Valid {
		c.Password = cfg.Password
	}
	if cfg.Version.Valid {
		c.Version = cfg.Version
	}
	if cfg.SSL {
		c.SSL = cfg.SSL
	}

	if c.Insecure {
		c.Insecure = cfg.Insecure
	}

	c.InfluxDBConfig = c.InfluxDBConfig.Apply(cfg.InfluxDBConfig)
	return c
}

// ParseArg takes an arg string and converts it to a config
func ParseArg(arg string) (Config, error) {
	c := Config{}
	params, err := strvals.Parse(arg)
	if err != nil {
		return c, err
	}

	if v, ok := params["brokers"].(string); ok {
		params["brokers"] = []string{v}
	}

	if v, ok := params["influxdb"].(map[string]interface{}); ok {
		influxConfig, err := influxdbParseMap(v)
		if err != nil {
			return c, err
		}
		c.InfluxDBConfig = c.InfluxDBConfig.Apply(influxConfig)
	}

	delete(params, "influxdb")

	if v, ok := params["push_interval"].(string); ok {
		err := c.PushInterval.UnmarshalText([]byte(v))
		if err != nil {
			return c, err
		}
	}

	if v, ok := params["version"].(string); ok {
		c.Version = null.StringFrom(v)
	}

	if v, ok := params["ssl"].(bool); ok {
		c.SSL = v
	}

	if v, ok := params["skip_cert_verify"].(bool); ok {
		c.Insecure = v
	}

	if v, ok := params["auth_mechanism"].(string); ok {
		c.AuthMechanism = null.StringFrom(v)
	}

	if v, ok := params["user"].(string); ok {
		c.User = null.StringFrom(v)
	}

	if v, ok := params["password"].(string); ok {
		c.Password = null.StringFrom(v)
	}

	var cfg config
	err = mapstructure.Decode(params, &cfg)
	if err != nil {
		return c, err
	}

	c.Brokers = cfg.Brokers
	c.Topic = null.StringFrom(cfg.Topic)
	c.Format = null.StringFrom(cfg.Format)

	return c, nil
}

// GetConsolidatedConfig combines {default config values + JSON config +
// environment vars + arg config values}, and returns the final result.
func GetConsolidatedConfig(jsonRawConf json.RawMessage, env map[string]string, arg string) (Config, error) {
	result := NewConfig()
	if jsonRawConf != nil {
		jsonConf := Config{}
		if err := json.Unmarshal(jsonRawConf, &jsonConf); err != nil {
			return result, err
		}
		result = result.Apply(jsonConf)
	}

	envConfig := Config{}
	if err := envconfig.Process("", &envConfig); err != nil {
		// TODO: get rid of envconfig and actually use the env parameter...
		return result, err
	}

	result = result.Apply(envConfig)

	if result.AuthMechanism.String != "none" && (!result.User.Valid || !result.Password.Valid) {
		return result, errors.New("user and password are required when auth mechanism is provided")
	}

	if arg != "" {
		urlConf, err := ParseArg(arg)
		if err != nil {
			return result, err
		}
		result = result.Apply(urlConf)
	}

	return result, nil
}
