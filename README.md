# xk6-output-kafka

This extension is extracted from the [k6 kafka output](https://github.com/grafana/k6/pull/2081) so it can be used with [xk6](https://github.com/grafana/xk6).

## Build

To build a `k6` binary with this extension, first ensure you have the prerequisites:

- [Go toolchain](https://go101.org/article/go-toolchain.html)
- Git

> :point_up:

1. Build with `xk6`:

```bash
xk6 build --with github.com/grafana/xk6-output-kafka
```

This will result in a `k6` binary in the current directory.

2. Run with the just built `k6`:

```bash
./k6 run -o xk6-kafka <script.js>
```

## Instructions

You can configure the broker (or multiple ones), topic, and message format directly from the command line parameter like this:

```bash
./k6 run --out xk6-kafka=brokers=broker_host:9092,topic=k6
```

or if you want multiple brokers:

```bash
./k6 --out xk6-kafka=brokers={broker1,broker2},topic=k6,format=json
```

You can also specify the message `format` k6 will use. By default, it will be the same as the JSON output, but you can also use the InfluxDB line protocol for direct "consumption" by InfluxDB:

```bash
./k6 --out xk6-kafka=brokers=someBroker,topic=someTopic,format=influxdb
```

You can even modify some of the `format` settings such as `tagsAsFields`:

```bash
./k6 --out xk6-kafka=brokers=someBroker,topic=someTopic,format=influxdb,influxdb.tagsAsFields={url,myCustomTag}
```

## Testing Locally
This repo includes a [docker-compose.yml](docker-compose.yml) file that starts local Kafka environment with several dependencies and utilities baked-in.
See [lensesio/fast-data-dev](https://github.com/lensesio/fast-data-dev) for more information.

> :warning: Be sure that you've already compiled your custom `k6` binary as described in the [Build](#build) section! 

We'll use this environment to run some examples.

1. Start the docker compose environment.

   ```shell
   docker compose up -d
   ```
   Once you see the following, you should be ready.
   ```shell
    [+] Running 2/2
    ⠿ Network xk6-output-kafka_default       Created
    ⠿ Container xk6-output-kafka-lensesio-1  Started
   ```
   Next, we'll use the `k6` binary we compiled in the [Build section](#build) above.

1. Open http://localhost:3030/ to display the landing page for your local [Kafka Development environment](http://localhost:3030/) provided by the docker image.
   From there, you can select the [Kafka Topics UI](http://localhost:3030/kafka-topics-ui/).

1. Using our custom `k6` binary, we can execute our [example script](examples/simple.js) outputting test metrics to kafka.
   ```shell
   ./k6 run --out xk6-kafka=brokers={localhost:9092},topic=k6-metrics examples/simple.js
   ``` 
   Refreshing your browser should now list the `k6-metrics` topic. 
   Clicking into the topic will now show messages for real-time metrics published during the `k6` test execution.

## See also

- [k6 Kafka Documentation](https://k6.io/docs/results-output/real-time/apache-kafka/)
- [Integrating k6 with Apache Kafka](https://k6.io/blog/integrating-k6-with-apache-kafka)
