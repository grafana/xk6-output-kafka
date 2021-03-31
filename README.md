# xk6-output-kafka
Is extracted the original [kafka output](https://k6.io/docs/results-visualization/apache-kafka) from [k6](https://github.com/loadimpact/k6) so it can be used with [xk6](https://github.com/k6io/xk6).

</div>

## Build

To build a `k6` binary with this extension, first ensure you have the prerequisites:

- [Go toolchain](https://go101.org/article/go-toolchain.html)
- Git

Then:

1. Download `xk6`:
  ```bash
  $ go get -u github.com/k6io/xk6
  ```

2. Build the binary:
  ```bash
  $ xk6 build --with github.com/k6io/xk6-output-kafka
  ```
