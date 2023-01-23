# xk6-output-kafka

This extension is extracted from the [k6 kafka output](https://github.com/grafana/k6/pull/2081) so it can be used with [xk6](https://github.com/grafana/xk6).

</div>

## Build

To build a `k6` binary with this extension, first ensure you have the prerequisites:

- [Go toolchain](https://go101.org/article/go-toolchain.html)
- Git

1. Build with `xk6`:

```bash
xk6 build --with github.com/grafana/xk6-output-kafka
```

This will result in a `k6` binary in the current directory.

2. Run with the just build `k6:

```bash
./k6 run -o xk6-kafka <script.js>
```

## Instructions

You can configure the broker (or multiple ones), topic and message format directly from the command line parameter like this:


```bash
k6 run --out xk6-kafka=brokers=broker_host:8000,topic=k6
```

or if you want multiple brokers:

```bash
k6 --out xk6-kafka=brokers={broker1,broker2},topic=k6,format=json
```

You can also specify the message `format` k6 will use. By default, it will be the same as the JSON output, but you can also use the InfluxDB line protocol for direct "consumption" by InfluxDB:


```bash
$ k6 --out xk6-kafka=brokers=someBroker,topic=someTopic,format=influxdb
```


You can even modify some of the `format` settings such as `tagsAsFields`:


```bash
$ k6 --out xk6-kafka=brokers=someBroker,topic=someTopic,format=influxdb,influxdb.tagsAsFields={url,myCustomTag}
```

</CodeGroup>

## See also

- [Integrating k6 with Apache Kafka](https://k6.io/blog/integrating-k6-with-apache-kafka)

