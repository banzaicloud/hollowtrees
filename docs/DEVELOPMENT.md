# Development guide

## Building and running Hollowtrees on your local machine

The project can be built with `make`, configuration is done via `config.yaml`.
Copy the `config.yaml.dist` file, or create a new one with custom action plugins and rules.

```bash
make build
./build/hollowtrees
```

## Setting up action plugins

Currently only the `grpc` plugin type is supported.

```bash
plugins:
  - name: "grpc-dummy"
    address: "localhost:9091"
    type: "grpc"
```

## Triggering the Hollowtrees server with an alert

The Hollowtrees server is triggered by Prometheus alerts in a Kubernetes deployment, but to test things out the API can be triggered with a simple `cURL` command that simulates an alert sent by Prometheus.
Save the following JSON in a file named `alert.json`, and run the `cURL` command below.
The `alertname` label is required, it will be converted to an event type that's used to select action flows for a specific event.
The other labels are optional and arbitrary, and all of the labels will be sent to the action plugins as `data`.

```json
[
  {
    "annotations": {},
    "startsAt": "2006-01-02T15:04:05Z",
    "endsAt":"2007-01-02T15:04:05Z",
    "generatorURL":"http://test",
    "labels": {
      "alertname": "TestAlert",
      "cluster_name":"test-cluster",
      "Name":"test"
    }
  }
]
```

```bash
curl -X POST -d @alert.json localhost:9092/api/v1/alerts
```

## Setting up an action flow for Hollowtrees

Here's an example config snippet that describes an action flow that can be triggered with the above JSON:

```bash
flows:
  test:
    name: "Test Flow"
    description: "test flow that triggers a grpc plugin if the event type is `prometheus.server.alert.TestAlert` and the cluster_name label matches `test-cluster`"
    plugins:
    - "dummy-plugin-1"
    allowedEvents:
    - "prometheus.server.alert.TestAlert"
    filters:
    - cluster_name: "test-cluster"
```

## Running a (dummy) gRPC action plugin

There is an example gRPC action plugin in `examples/grpc_plugin`. Enter that directory, build the plugin with `go build .` and run the binary.
The dummy plugin accepts every event type and logs the requests it gets through gRPC.
