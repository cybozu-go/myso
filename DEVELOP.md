# How to develop MOCO

## Running tests

MOCO has the following 4 kinds of tests:

1. Tests that do not depend on MySQL or Kubernetes
2. `pkg/dbop` tests that depend on MySQL version
3. Tests that depend on Kubernetes and therefore run by controller-runtime's envtest
4. End-to-end tests

To run these tests, use the following make targets respectively:

1. `make test`
2. `make test-dbop`
3. `make envtest`
4. Read [`e2e/README.md`](e2e/README.md)

## Generated files

Some files in the repository are auto-generated.

- [`docs/crd_mysqlcluster.md`](docs/crd_mysqlcluster.md) is generated by `make apidoc`.
- Some files under `config` are generated by `make manifests`.
- `api/**/*.deepcopy.go` are generated by `make generate`.

CI checks and fails if they need to be rebuilt.

## Testing with unreleased moco-agent

MOCO depends on [moco-agent][] that is released from a different repository.
The dependency is therefore managed in `go.mod` file.

To run e2e tests with an unreleased moco-agent, follow the instructions in
[`e2e/README.md`](e2e/README.md).

## Updating moco-agent

Run `go get github.com/cybozu-go/moco-agent@latest`.

## Updating fluent-bit

Edit `FluentBitImage` in [`version.go`](versoin.go).

[moco-agent]: https://github.com/cybozu-go/moco-agent