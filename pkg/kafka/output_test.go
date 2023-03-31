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
	"fmt"
	"os"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
	"gopkg.in/guregu/null.v3"
)

func TestRun(t *testing.T) {
	broker := sarama.NewMockBroker(t, 1)
	coordinator := sarama.NewMockBroker(t, 2)
	seedMeta := new(sarama.MetadataResponse)
	seedMeta.AddBroker(coordinator.Addr(), coordinator.BrokerID())
	seedMeta.AddTopicPartition("my_topic", 0, 1, []int32{}, []int32{}, []int32{}, sarama.ErrNoError)
	broker.Returns(seedMeta)

	os.Clearenv()

	c, err := New(output.Params{
		Logger:     testutils.NewLogger(t),
		JSONConfig: json.RawMessage(fmt.Sprintf(`{"brokers":[%q], "topic": "my_topic", "version": "0.8.2.0"}`, broker.Addr())),
	})

	require.NoError(t, err)

	require.NoError(t, c.Start())
	require.NoError(t, c.Stop())
}

func TestFormatSample(t *testing.T) {
	o := Output{}
	registry := metrics.NewRegistry()

	metric, err := registry.NewMetric("my_metric", metrics.Gauge)
	require.NoError(t, err)

	samples := make(metrics.Samples, 2)
	samples[0] = metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: metric,
			Tags: registry.RootTagSet().WithTagsFromMap(map[string]string{
				"a": "1",
			}),
		},
		Value: 1.25,
	}
	samples[1] = metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: metric,
			Tags: registry.RootTagSet().WithTagsFromMap(map[string]string{
				"b": "2",
			}),
		},
		Value: 2,
	}

	o.Config.Format = null.NewString("influxdb", false)
	formattedSamples, err := o.formatSamples(samples)

	assert.Nil(t, err)
	assert.Equal(t, []string{"my_metric,a=1 value=1.25", "my_metric,b=2 value=2"}, formattedSamples)

	o.Config.Format = null.NewString("json", false)
	formattedSamples, err = o.formatSamples(samples)

	expJSON1 := "{\"type\":\"Point\",\"data\":{\"time\":\"0001-01-01T00:00:00Z\",\"value\":1.25,\"tags\":{\"a\":\"1\"}},\"metric\":\"my_metric\"}"
	expJSON2 := "{\"type\":\"Point\",\"data\":{\"time\":\"0001-01-01T00:00:00Z\",\"value\":2,\"tags\":{\"b\":\"2\"}},\"metric\":\"my_metric\"}"

	assert.Nil(t, err)
	assert.Equal(t, []string{expJSON1, expJSON2}, formattedSamples)
}
