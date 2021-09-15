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
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	"github.com/kelseyhightower/envconfig"
	"github.com/kubernetes/helm/pkg/strvals"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib/types"
)

// Config is the config for the kafka collector
type Config struct {
	// Connection.
	Brokers []string `json:"brokers" envconfig:"K6_KAFKA_BROKERS"`

	// Samples.
	Topic                 null.String        `json:"topic" envconfig:"K6_KAFKA_TOPIC"`
	User                  null.String        `json:"user" envconfig:"K6_KAFKA_SASL_USER"`
	Password              null.String        `json:"password" envconfig:"K6_KAFKA_SASL_PASSWORD"`
	AuthMechanism         null.String        `json:"authMechanism" envconfig:"K6_KAFKA_AUTH_MECHANISM"`
	Format                null.String        `json:"format" envconfig:"K6_KAFKA_FORMAT"`
	PushInterval          types.NullDuration `json:"pushInterval" envconfig:"K6_KAFKA_PUSH_INTERVAL"`
	Version               null.String        `json:"version" envconfig:"K6_KAFKA_VERSION"`
	SSL                   null.Bool          `json:"ssl" envconfig:"K6_KAFKA_SSL"`
	InsecureSkipTLSVerify null.Bool          `json:"insecureSkipTLSVerify" envconfig:"K6_KAFKA_INSECURE_SKIP_TLS_VERIFY"`
	LogError              null.Bool          `json:"logError" envconfig:"K6_KAFKA_LOG_ERROR"`

	InfluxDBConfig influxdbConfig `json:"influxdb"`
}

// NewConfig creates a new Config instance with default values for some fields.
func NewConfig() Config {
	return Config{
		Format:                null.StringFrom("json"),
		PushInterval:          types.NullDurationFrom(1 * time.Second),
		InfluxDBConfig:        newInfluxdbConfig(),
		AuthMechanism:         null.StringFrom("none"),
		Version:               null.StringFrom(sarama.DefaultVersion.String()),
		SSL:                   null.BoolFrom(false),
		InsecureSkipTLSVerify: null.BoolFrom(false),
		LogError:              null.BoolFrom(true),
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
	if cfg.SSL.Valid {
		c.SSL = cfg.SSL
	}

	if cfg.InsecureSkipTLSVerify.Valid {
		c.InsecureSkipTLSVerify = cfg.InsecureSkipTLSVerify
	}

	if cfg.LogError.Valid {
		c.LogError = cfg.LogError
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

	if v, ok := params["influxdb"].(map[string]interface{}); ok {
		influxConfig, err := influxdbParseMap(v)
		if err != nil {
			return c, err
		}
		c.InfluxDBConfig = c.InfluxDBConfig.Apply(influxConfig)
	}

	delete(params, "influxdb")

	if v, ok := params["pushInterval"].(string); ok {
		err := c.PushInterval.UnmarshalText([]byte(v))
		if err != nil {
			return c, err
		}
		delete(params, "pushInterval")
	}

	if v, ok := params["version"].(string); ok {
		c.Version = null.StringFrom(v)
		delete(params, "version")
	}

	if v, ok := params["ssl"].(bool); ok {
		c.SSL = null.BoolFrom(v)
		delete(params, "ssl")
	}

	if v, ok := params["insecureSkipTLSVerify"].(bool); ok {
		c.InsecureSkipTLSVerify = null.BoolFrom(v)
		delete(params, "insecureSkipTLSVerify")
	}

	if v, ok := params["logError"].(bool); ok {
		c.LogError = null.BoolFrom(v)
		delete(params, "logError")
	}

	if v, ok := params["authMechanism"].(string); ok {
		c.AuthMechanism = null.StringFrom(v)
		delete(params, "authMechanism")
	}

	if v, ok := params["user"].(string); ok {
		c.User = null.StringFrom(v)
		delete(params, "user")
	}

	if v, ok := params["password"].(string); ok {
		c.Password = null.StringFrom(v)
		delete(params, "password")
	}
	if v, ok := params["topic"].(string); ok {
		c.Topic = null.StringFrom(v)
		delete(params, "topic")
	}
	if v, ok := params["format"].(string); ok {
		c.Format = null.StringFrom(v)

		delete(params, "format")
	}

	if v, ok := params["brokers"].(string); ok {
		c.Brokers = []string{v}

		delete(params, "brokers")
	}
	if v, ok := params["brokers"].([]interface{}); ok {
		c.Brokers = interfaceSliceToStringSlice(v)
		delete(params, "brokers")
	}

	if len(params) > 0 {
		return c, errors.New("Unknown or unparsed options '" + mapToString(params) + "'")
	}
	return c, nil
}

func mapToString(m map[string]interface{}) string {
	var s string
	for k, v := range m {
		s += fmt.Sprintf("%s=%v,", k, v)
	}
	return s[:len(s)-1]
}

func interfaceSliceToStringSlice(input []interface{}) []string {
	output := make([]string, len(input))
	for i, v := range input {
		output[i] = fmt.Sprintf("%v", v)
	}
	return output
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
