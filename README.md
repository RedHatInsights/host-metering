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
* mocked `subscriptin-manager`
* test configuration file that is lowering intervals and using the following:
* test certificate
* custom path for metrics WAL
* Prometheus server started in a podman container

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
