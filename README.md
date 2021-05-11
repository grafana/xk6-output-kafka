# xk6-output-kafka
Is extracted the original [kafka output](https://k6.io/docs/results-visualization/apache-kafka) from [k6](https://github.com/k6io/k6) so it can be used with [xk6](https://github.com/k6io/xk6).

</div>

## Build

To build a `k6` binary with this extension, first ensure you have the prerequisites:

- [Go toolchain](https://go101.org/article/go-toolchain.html)
- Git

1. Build with `xk6`:

```bash
xk6 build --with github.com/k6io/xk6-output-kafka
```

This will result in a `k6` binary in the current directory.

2. Run with the just build `k6:

```bash
./k6 run -o xk6-kafka <script.js>
```

## Configuration

The configuration is the same as the in k6 one. Please consult the [documentation](https://k6.io/docs/results-visualization/apache-kafka) for specifics.
