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

rpms via mock:

```
$ make mock   # EPEL 7
$ make mock-8 # CentOS Stream 8
$ make mock-9 # CentOS Stream 9
```

rpms directly via `rpmbuild`

```
$ make rpm
```

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

Note: You may specify the UBI version you wish to test with the optional `UBI_VERSION` argument. If not specified, it will default to 7:

```
$ make test-daemon UBI_VERSION=<version_number>
```

Query Prometheus, e.g., via command:

```
$ curl 'http://localhost:9090/api/v1/query?query=system_cpu_logical_count' | jq
```

Or visit the Prometheus Web UI at http://localhost:9090/graph?g0.expr=system_cpu_logical_count&g0.tab=0&g0.range_input=1m

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

