# Go template

> Go template for new projects

This repository is a template to make use of when creating new projects in
Go. It contains scripts, Dockerfile(s) and workflows.

* [Module](#module)
* [Service](#service)
* [Scripts](#scripts)
* [Dockerfile](#dockerfile)
* [Workflows](#workflows)

## Module

The module and its imports needs to be updated.

1. Update `go.mod` with the correct module name.
2. Update imports to the new module.

## Service

The template contains a simple generic foundation for creating a service that can be used as a starting point. The `main.go` has the bare minimum to start, if need be, update `main.go` with additional setup code from a `config` package or other means of configuration.

The service implementation, startup and shutdown logic must be implemented.

## Scripts

### `init.sh`

Initialises the project by performing replacements in the Dockerfile with the
provided parameters for application name and port.

The script removes itself after it has been run.

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

The Dockerfile needs the following updated:

* `ARG BIN` needs to be updated from `ARG BIN={{bin}}` to `ARG BIN=<application-name>`.
* `ENTRYPOINT` needs to be updated from `ENTRYPOINT [ "/{{bin}}" ]`
  to `ENTRYPOINT [ "/<application-name>" ]`..

These steps can be performed by running `scripts/init.sh`.

If `ca-certificates` is not needed by the project, the following lines can be deleted:

* `RUN apk update && apk add --no-cache ca-certificates && update-ca-certificates` from the **First step**.
* `COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/` from the **Second step**.

## Workflows

### `test.yaml`

Workflow to be called by other workflows, runs a job with the most common steps for testing a Go module.

### `build.yaml`

Workflow that includes a call to the `test.yaml` workflow. A starter workflow that needs to be modified to suit the project.
