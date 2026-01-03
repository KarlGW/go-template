# Go template

> Go template for new projects

This repository is a template to make use of when creating new projects in
Go. It contains scripts, Dockerfile(s) and workflows.

* [Module](#module)
* [HTTP server](#http-server)
  * [Routes](#routes)
  * [Logging](#logging)
* [Scripts](#scripts)
* [Dockerfile](#dockerfile)
* [Workflows](#workflows)

## HTTP server

The template contains a simple HTTP server that can be used as a starting point. The `main.go` has the bare minimum to start, if need be, update `main.go` with additional setup code from a `config` package or other means of configuration.

### Routes

Routes should be added in the `server` method `routes` in `server/routes.go`. Exchanging the router for another implemention is straight forward.
It can be done by updating the `server` struct field `router`, the construction function `New` and the `Options` struct.

### Logging

A basic request logger middleware is made available in the file `server/middleware_logger.go`. It can be used as follows:

**Standard library**

```go
s.router.Handle("/", newRequestLogger(handler()))

func (s server) newRequestLogger(next http.Handler) http.Handler {
  return requestLogger(s.log, next)
}
```

**`chi`**

```go
s.router.Use(newRequestLogger)

s.router.Handle("/", s.handler())

func (s server) newRequestLogger(next http.Handler) http.Handler {
  return requestLogger(s.log, next)
}
```

## Scripts

### `release.sh`

Script to prepare a release. The script makes sure the current branch is the base branch (often `main`), pulls from remote, run tests and then finally creates a Git tag with the version number.

**Usage**

```sh
# Set a version (valid semver without the 'v' prefix).
./scripts/bash/release.sh --version <version>

# If there already is existing tagged versions, the following can be used.

# Patch increment (patch number according to semantic versioning).
./scripts/bash/release.sh --patch

# Minor increment (minor number according to semantic versioning).
./scripts/bash/release.sh --minor

# Major increment (minor number according to semantic versioning).
./scripts/bash/release.sh --major
```

## Dockerfile

**Note**: The Dockerfile needs the following updated:

* `ARG BIN` needs to be updated `ARG BIN=<binary-name>` (if not provided during build).
* `ARG PORT` needs to be updated to `ARG PORT=<port-number>` (if not provided during build).

* `ENTRYPOINT` needs to be updated to `ENTRYPOINT [ "/<binary-name>" ]`.

If `ca-certificates` is not needed by the project, the following lines can be deleted:

* `RUN apk update && apk add --no-cache ca-certificates && update-ca-certificates` from the **First step**.
* `COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/` from the **Second step**.

## Workflows

### `test.yaml`

Workflow to be called by other workflows, runs a job with the most common steps for testing a Go module.

### `build.yaml`

Workflow that includes a call to the `test.yaml` workflow. A starter workflow that needs to be modified to suit the project.
