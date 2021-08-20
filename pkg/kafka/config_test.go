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
	"os"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib/types"
)

func TestConfigParseArg(t *testing.T) {
	c, err := ParseArg("brokers=broker1,topic=someTopic,format=influxdb")
	expInfluxConfig := influxdbConfig{}
	assert.Nil(t, err)
	assert.Equal(t, []string{"broker1"}, c.Brokers)
	assert.Equal(t, null.StringFrom("someTopic"), c.Topic)
	assert.Equal(t, null.StringFrom("influxdb"), c.Format)
	assert.Equal(t, expInfluxConfig, c.InfluxDBConfig)

	c, err = ParseArg("brokers={broker2,broker3:9092},topic=someTopic2,format=json")
	assert.Nil(t, err)
	assert.Equal(t, []string{"broker2", "broker3:9092"}, c.Brokers)
	assert.Equal(t, null.StringFrom("someTopic2"), c.Topic)
	assert.Equal(t, null.StringFrom("json"), c.Format)

	c, err = ParseArg("brokers={broker2,broker3:9092},topic=someTopic,format=influxdb,influxdb.tagsAsFields=fake")
	expInfluxConfig = influxdbConfig{
		TagsAsFields: []string{"fake"},
	}
	assert.Nil(t, err)
	assert.Equal(t, []string{"broker2", "broker3:9092"}, c.Brokers)
	assert.Equal(t, null.StringFrom("someTopic"), c.Topic)
	assert.Equal(t, null.StringFrom("influxdb"), c.Format)
	assert.Equal(t, expInfluxConfig, c.InfluxDBConfig)

	c, err = ParseArg("brokers={broker2,broker3:9092},topic=someTopic,format=influxdb,influxdb.tagsAsFields={fake,anotherFake}")
	expInfluxConfig = influxdbConfig{
		TagsAsFields: []string{"fake", "anotherFake"},
	}
	assert.Nil(t, err)
	assert.Equal(t, []string{"broker2", "broker3:9092"}, c.Brokers)
	assert.Equal(t, null.StringFrom("someTopic"), c.Topic)
	assert.Equal(t, null.StringFrom("influxdb"), c.Format)
	assert.Equal(t, expInfluxConfig, c.InfluxDBConfig)

	c, err = ParseArg("brokers={broker2,broker3:9092},topic=someTopic,format=json,auth_mechanism=SASL_PLAINTEXT,user=johndoe,password=123password")
	assert.Nil(t, err)
	assert.Equal(t, []string{"broker2", "broker3:9092"}, c.Brokers)
	assert.Equal(t, null.StringFrom("someTopic"), c.Topic)
	assert.Equal(t, null.StringFrom("json"), c.Format)
	assert.Equal(t, null.StringFrom("SASL_PLAINTEXT"), c.AuthMechanism)
	assert.Equal(t, null.StringFrom("johndoe"), c.User)
	assert.Equal(t, null.StringFrom("123password"), c.Password)
	assert.Equal(t, false, c.SSL)

	c, err = ParseArg("brokers={broker2,broker3:9092},topic=someTopic,format=json,auth_mechanism=SASL_PLAINTEXT,user=johndoe,password=123password,ssl=false")
	assert.Nil(t, err)
	assert.Equal(t, []string{"broker2", "broker3:9092"}, c.Brokers)
	assert.Equal(t, null.StringFrom("someTopic"), c.Topic)
	assert.Equal(t, null.StringFrom("json"), c.Format)
	assert.Equal(t, null.StringFrom("SASL_PLAINTEXT"), c.AuthMechanism)
	assert.Equal(t, null.StringFrom("johndoe"), c.User)
	assert.Equal(t, null.StringFrom("123password"), c.Password)
	assert.Equal(t, false, c.SSL)

	c, err = ParseArg("brokers={broker2,broker3:9092},topic=someTopic,format=json,auth_mechanism=SASL_PLAINTEXT,user=johndoe,password=123password,ssl=true")
	assert.Nil(t, err)
	assert.Equal(t, []string{"broker2", "broker3:9092"}, c.Brokers)
	assert.Equal(t, null.StringFrom("someTopic"), c.Topic)
	assert.Equal(t, null.StringFrom("json"), c.Format)
	assert.Equal(t, null.StringFrom("SASL_PLAINTEXT"), c.AuthMechanism)
	assert.Equal(t, null.StringFrom("johndoe"), c.User)
	assert.Equal(t, null.StringFrom("123password"), c.Password)
	assert.Equal(t, true, c.SSL)
}

func TestConsolidatedConfig(t *testing.T) {
	t.Parallel()
	// TODO: add more cases
	testCases := map[string]struct {
		jsonRaw json.RawMessage
		env     map[string]string
		arg     string
		config  Config
		err     string
	}{
		"default": {
			env: map[string]string{
				"K6_KAFKA_AUTH_MECHANISM": "none",
			},
			config: Config{
				Format:         null.StringFrom("json"),
				PushInterval:   types.NullDurationFrom(1 * time.Second),
				InfluxDBConfig: newInfluxdbConfig(),
				AuthMechanism:  null.StringFrom("none"),
				Version:        null.StringFrom(sarama.DefaultVersion.String()),
			},
		},
		"auth": {
			env: map[string]string{
				"K6_KAFKA_AUTH_MECHANISM": "scram-sha-512",
				"K6_KAFKA_SASL_PASSWORD":  "password123",
				"K6_KAFKA_SASL_USER":      "testuser",
			},
			config: Config{
				Format:         null.StringFrom("json"),
				PushInterval:   types.NullDurationFrom(1 * time.Second),
				InfluxDBConfig: newInfluxdbConfig(),
				AuthMechanism:  null.StringFrom("scram-sha-512"),
				Password:       null.StringFrom("password123"),
				User:           null.StringFrom("testuser"),
				Version:        null.StringFrom(sarama.DefaultVersion.String()),
			},
		},
		"auth-missing-credentials": {
			env: map[string]string{
				"K6_KAFKA_AUTH_MECHANISM": "scram-sha-512",
			},
			config: Config{
				Format:         null.StringFrom("json"),
				PushInterval:   types.NullDurationFrom(1 * time.Second),
				InfluxDBConfig: newInfluxdbConfig(),
				AuthMechanism:  null.StringFrom("scram-sha-512"),
				Version:        null.StringFrom(sarama.DefaultVersion.String()),
			},
			err: "user and password are required when auth mechanism is provided",
		},
		"auth-missing-user": {
			env: map[string]string{
				"K6_KAFKA_AUTH_MECHANISM": "scram-sha-512",
				"K6_KAFKA_SASL_PASSWORD":  "password123",
			},
			config: Config{
				Format:         null.StringFrom("json"),
				PushInterval:   types.NullDurationFrom(1 * time.Second),
				InfluxDBConfig: newInfluxdbConfig(),
				AuthMechanism:  null.StringFrom("scram-sha-512"),
				Version:        null.StringFrom(sarama.DefaultVersion.String()),
			},
			err: "user and password are required when auth mechanism is provided",
		},
		"auth-missing-password": {
			env: map[string]string{
				"K6_KAFKA_AUTH_MECHANISM": "scram-sha-512",
				"K6_KAFKA_SASL_USER":      "testuser",
			},
			config: Config{
				Format:         null.StringFrom("json"),
				PushInterval:   types.NullDurationFrom(1 * time.Second),
				InfluxDBConfig: newInfluxdbConfig(),
				AuthMechanism:  null.StringFrom("scram-sha-512"),
				Version:        null.StringFrom(sarama.DefaultVersion.String()),
			},
			err: "user and password are required when auth mechanism is provided",
		},
	}

	for name, testCase := range testCases {
		testCase := testCase
		t.Run(name, func(t *testing.T) {
			// hacks around env not actually being taken into account
			os.Clearenv()
			defer os.Clearenv()
			for k, v := range testCase.env {
				require.NoError(t, os.Setenv(k, v))
			}

			config, err := GetConsolidatedConfig(testCase.jsonRaw, testCase.env, testCase.arg)
			if testCase.err != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), testCase.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, testCase.config, config)
		})
	}
}
