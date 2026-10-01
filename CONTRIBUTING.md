# Contributing to turn

turn follows the Exalynt [contributing guide](https://github.com/exalynt/community/blob/main/CONTRIBUTING.md) and [Code of Conduct](https://github.com/exalynt/community/blob/main/CODE_OF_CONDUCT.md). Read those first: they cover issues, pull requests, code review, AI-assisted contributions, security, and licensing. This document adds only what is specific to turn.

## Stability

turn is in **Alpha**, one of Exalynt's [stability levels](https://readme.exalynt.com/how-it-works/stability-levels). It works, but it is still taking shape, so breaking changes are expected and don't need a new version.

Label each issue and pull request with the stability level it targets, as described in [Marking stability levels](https://readme.exalynt.com/engineers/marking-stability). Most work is labeled `stability: alpha` for now. A pull request that moves turn to a new level says so in its description.

## Development setup

You need:

- Go at the version in `go.mod` or newer.
- [golangci-lint](https://golangci-lint.run/) v2.9.0, the version CI pins.

There is nothing else to install to build turn or run its unit tests: turn has no dependencies outside the standard library.

### Test databases

Integration tests check that what turn hands back produces correct pages on real stores. They need [Docker](https://docs.docker.com/get-docker/) with Compose v2. `compose.yml` runs one container per database:

| Service    | Image                | Connection from the host                                     |
| ---------- | -------------------- | ------------------------------------------------------------ |
| `postgres` | `postgres:18-alpine` | `postgres://turn:turn@localhost:5432/turn?sslmode=disable`   |
| `mysql`    | `mysql:8.4`          | `turn:turn@tcp(localhost:3306)/turn`                         |
| `mongo`    | `mongo:8`            | `mongodb://turn:turn@localhost:27017/turn?authSource=admin`  |

Compose doesn't publish any ports by default. Copy the example override to publish them on the host ports above:

```sh
cp compose.override.example.yml compose.override.yml
make up     # start the databases and wait until they are healthy
make ps     # show each database and its health
make down   # stop them
make nuke   # stop them and delete their data
```

`compose.override.yml` is gitignored, so you can change it freely. If a port is already in use, for example by another project's Postgres on 5432, change the left-hand number in your override and use that port in the connection string.

Each database keeps its data on a named volume, so data survives `make down`. Write tests that create and clean up their own tables or collections rather than relying on what's already there. Use `make nuke` to start over.

## Checks

Before opening a pull request, run everything CI checks:

```sh
make check   # go fix, go fmt, go vet, golangci-lint
make test    # go test ./...
```

CI runs build, vet, test, gofmt, and golangci-lint on every pull request and on `main`. CI uses the Go version in `go.mod`, so code that needs a newer Go fails there even if it passes locally. Run `make help` to see every target.

## Design principles

Keep these in mind when proposing a change. A change that departs from them should be discussed in an issue first.

- **Standard library only.** turn is meant to be imported anywhere without pulling in anything else. A new dependency needs a very strong case, beyond what the community guide's [dependency guidance](https://github.com/exalynt/community/blob/main/CONTRIBUTING.md#dependencies) already asks.
- **Storage-agnostic.** turn hands consumers what a query needs: a limit, an offset, or the key to page after. It never builds SQL or touches a database driver. Adapters for a specific store belong in the consuming project.
- **Transport-light.** Reading pagination parameters from a URL query is in scope. Anything that's tied to a particular router or framework belongs in the consumer.
- **Small surface.** Each exported identifier is something we have to keep supporting. Prefer one general building block over several convenience variants.
- **Errors callers can branch on.** Report invalid input through exported sentinel errors, so a handler can map it to a 400 with `errors.Is`. Don't return ad hoc error strings for input validation.

## Compatibility

While turn is in Alpha, breaking changes are allowed, but call them out in the pull request description so consumers know what to change. Once turn reaches GA, breaking changes go through a new major version, per [Breaking changes after GA](https://readme.exalynt.com/how-it-works/stability-levels#breaking-changes-after-ga).

turn is a library, so changes reach consumers on their next upgrade, sometimes while requests are still in flight. Besides the Go API itself, treat all of these as public:

- **JSON field names** on response types. Consumers serialize these straight into API responses, so renaming a field breaks their clients.
- **Query parameter names** read from requests.
- **Cursor encoding.** Cursors are handed to end users and come back later, possibly after the service has been upgraded. If the encoding changes, a cursor issued by the previous release must still decode, or the change must be called out as breaking.
- **Defaults,** such as the default and maximum page size.
- **Minimum Go version.** Raising the `go` directive forces every consumer to upgrade too. Raise it only when a feature needs it, and say so in the pull request.

turn follows [semantic versioning](https://semver.org/).

## Documentation

The package documentation in `doc.go` and the usage in `README.md` are the main way consumers learn turn. If you change how a type is meant to be used, update both. Doc comments should state behavior a caller can rely on, such as edge cases, zero values, and which errors are returned, rather than describe how it is implemented.
