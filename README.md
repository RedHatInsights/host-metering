# Host Metering

Host metering client.

## Usage

```
$ host-metering daemon
```

Or if run from RPMs as a service:

```
# yum install host-metering
# systemctl enable host-metering
# systemctl start host-metering
```

See output/log:

```
# journalctl -feu host-metering
```

## RPM repository

RPM builds of `main` branch are available at COPR:  https://copr.fedorainfracloud.org/coprs/pvoborni/host-metering/

## Contribute

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Build

```
$ go build
```

rpms via `rpmbuild` (cross-distro builds are covered by packit/Copr CI):

```
$ make rpm
```

The devcontainer image carries the full RPM build toolchain (Go, rpmbuild,
and the SELinux policy build). It is based on CentOS Stream 10 by default,
because `selinux-policy-devel` is not available in the UBI repositories.
To build the RPMs inside the devcontainer image:

```
$ make podman-containers
$ podman run --rm -v "$PWD:/workspace/host-metering:Z" \
    devcontainer_host-metering make -C /workspace/host-metering rpm
```

The resulting RPMs are written to `dist/rpmbuild/RPMS/`.

## Testing

### Unit tests

To run go unit test:
```
$ make test
```

### Local run / development

```
$ make test-daemon
```

It will run `host-metering` with:
* mocked `subscription-manager`
* test configuration file that is lowering intervals and using the following:
* test certificate
* custom path for metrics WAL
* Prometheus server started in a podman container

Note: You may specify the base image used for the devcontainer with the optional `BASE_IMAGE` argument (any el-family image carrying the build and test dependencies works; `selinux-policy-devel` rules out the UBI images). If not specified, it will default to CentOS Stream 10:

```
$ make test-daemon BASE_IMAGE=quay.io/centos/centos:stream9
```

Query Prometheus, e.g., via command:

```
$ curl 'http://localhost:9090/api/v1/query?query=system_cpu_logical_count' | jq
```

Or visit the Prometheus Web UI at http://localhost:9090/graph?g0.expr=system_cpu_logical_count&g0.tab=0&g0.range_input=1m

### Integration tests

The integration suite runs the daemon against a local Prometheus
remote-write receiver and asserts metric collection, WAL backlog recovery,
certificate hot-reload, daemon lifecycle behavior, and that unregistered
hosts send nothing. It runs in a test container against a real
subscription-manager registered to a containerized Candlepin server - the
suite does not mock subscription-manager (the mocks in `mocks/` are for
the unit tests). It needs a compose tool (podman-compose, docker compose,
or docker-compose) and curl on the host:

```
$ integration-tests/run-tests.sh --integration            # default: UBI 10
$ integration-tests/run-tests.sh --ubi-version 9           # specific major
$ integration-tests/run-tests.sh --unit                    # Go unit tests
$ integration-tests/run-tests.sh --all                     # unit + integration
```

The Candlepin server, credentials, and organization can be overridden via
the `CANDLEPIN_HOST`, `CANDLEPIN_PORT`, `CANDLEPIN_USERNAME`,
`CANDLEPIN_PASSWORD`, and `CANDLEPIN_ORG` environment variables (defaults
match the `ghcr.io/candlepin/candlepin-app` test containers in
`integration-tests/docker/docker-compose.test.yml`).

Cross-major validation (real RHEL VMs, RPM-installed builds) will run on
Testing Farm via plans added by a follow-up change.

### Clean-up

```
$ make clean      # clean build&test files
$ make clean-pod  # destroy podman pod
```

## Running in a container
This project has configuration for running inside VSCode container.
### Prerequisites:
1. It requires podman and podman-compose to be installed on the host machine.
To install podman-compose, please run
```
sudo dnf install podman-compose
```
2. Make sure to add the following settings to user's settings.json (`ctrl+shift+P` -> `Preferences: Open user settings (JSON)`). This will set up the dev containers plugin to work with `podman` and `podman-compose`
```
    "dev.containers.dockerComposePath": "podman-compose",
    "dev.containers.dockerPath": "podman"
```
3. Execute `.devcontainer/commands/prepare_containers.sh`. It will create `docker-compose.local.yml` file that will be used to run the container properly.

### Running make commands in a container
There is an option to run make commands inside the `docker-compose` generated environment. Just prefix a make command you would like to run with `podman-`. e.g. to run `make test` in a container, use `make podman-test`.

## Mocking subscription-manager commands
`mocked_run.sh` is a shortcut to running `go run main.go` with  mocked context.

### Preparing the mocks
Inside the `mocks` folder run `./mock_from_host.sh` to generate outputs that will be used as mocks.
You have to have a system with `subscription-manager` installed and registered correctly to generate the mocks.

