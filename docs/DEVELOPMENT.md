## Development guide

#### Building and running Hollowtrees on your local machine

The project can be built with `make`, configuration is done via `conf/config.yaml`.
Copy the `conf/config.yaml.example` file, or create a new one with custom action plugins and rules.
```
make build
./hollowtrees
```

#### Setting up action plugins

Two type of action plugins are supported: `grpc` and `fn`.
An `fn` action plugin is basically an `fn` function that have a corresponding fn app and route, these must be provided along the `fn` server's address.

```
action_plugins:
  - name: "grpc-dummy"
    address: "localhost:9093"
    type: "grpc"

  - name: "fn-sync-test"
    address: "localhost:8080"
    type: "fn"
    properties:
      app: "myapp"
      function: "hello"

  - name: "fn-async-test"
    address: "localhost:8080"
    type: "fn"
    properties:
      app: "myasyncapp"
      function: "hello-async"
```

#### Triggering the Hollowtrees server with an alert

The Hollowtrees server is triggered by Prometheus alerts in a Kubernetes deployment, but to test things out the API can be triggered with a simple `cURL` command that simulates an alert sent by Prometheus.
Save the following JSON in a file named `alert.json`, and run the `cURL` command below.
The `alertname` label is required, it will be converted to an event type that's used to select action flows for a specific event.
The other labels are optional and arbitrary, and all of the labels will be sent to the action plugins as `data`.
```
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

```
curl -X POST -d @alert.json localhost:9092/api/v1/alerts
```

#### Setting up an action flow for Hollowtrees

Here's an example rule that describes an action flow that can be triggered with the above JSON:

```
- name: "test_flow"
    description: "test flow that triggers a grpc plugin, a sync, and an async fn function sequentially if the cluster_name label matches test-cluster"
    event_type: "prometheus.server.alert.TestAlert"
    action_plugins:
      - "grpc-dummy"
      - "fn-sync-test"
      - "fn-async-test"
    match:
      - cluster_name: "test-cluster"
```

#### Running a (dummy) gRPC action plugin

There is an example gRPC action plugin in `dummy_plugin`. Enter that directory, build the plugin with `go build .` and run the binary.
The dummy plugin accepts every event type and logs the requests it gets through gRPC.

#### Running an fn action plugin

The easiest way to have an `fn` action plugin is to run an `fn` server on your local machine and deploy some test functions to it.
The fn tutorial on Github describes how to install `fn` locally and how to start a `hello world` function.

*Note*: for some reason the `fn` example was not working for me out of the box, and the docker image couldn't be built.
This is the error I got during the build:

```
fn --verbose deploy --app myapp --local
Deploying hello-go-async to app: myapp at path: /hello-async
Bumped to version 0.0.2
Building image martonsereg/hello-go-async:0.0.2
Sending build context to Docker daemon  6.144kB
Step 1/10 : FROM fnproject/go:dev as build-stage
 ---> fac877f7d14d
Step 2/10 : WORKDIR /function
Removing intermediate container 7ad6d896ac5a
 ---> dd52338a1fe7
Step 3/10 : RUN go get -u github.com/golang/dep/cmd/dep
 ---> Running in eebff45a95f0
# github.com/golang/dep/cmd/dep
2018/03/07 15:26:39 readSym out of sync
The command '/bin/sh -c go get -u github.com/golang/dep/cmd/dep' returned a non-zero code: 2
```

If you have a similar error when deploying/running the fn example, try to change the base docker image it uses by adding these two lines to the `func.yaml` file:

```
build_image: golang:1.9
run_image: golang:1.9
```