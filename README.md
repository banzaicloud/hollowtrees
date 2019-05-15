# Hollowtrees

_Hollowtrees is a wave for the highest level, the pin-up centrefold for the Mentawai islands bringing a new machine-like level to the word perfection. Watch out for the vigilant guardian aptly named The Surgeons Table, whose sole purpose is to take parts of you as a trophy._

_Hollowtrees, a ruleset based watchguard is keeping spot/preemptible instance based clusters safe and allows to use them in production.
Handles spot price surges within one region or availability zone and reschedules applications before instances are taking down. Hollowtrees follows the "batteries included but removable" principle and has plugins for different runtimes and frameworks. At the lowest level it manages spot based clusters of virtual machines, however it contains plugins for Kubernetes, Prometheus and Pipeline as well._

Hollowtrees is a core building block of the Pipeline platform. Check out the developer beta:
<p align="center">
  <a href="https://beta.banzaicloud.io">
  <img src="https://camo.githubusercontent.com/a487fb3128bcd1ef9fc1bf97ead8d6d6a442049a/68747470733a2f2f62616e7a6169636c6f75642e636f6d2f696d672f7472795f706970656c696e655f627574746f6e2e737667">
  </a>
</p>


**Warning:** _Hollowtrees is experimental, under development and does not have a stable release yet. If in doubt, don't go out._

## Quick start

Building the project is as simple as running a go build command. The result is a statically linked executable binary.

```bash
make build
./build/hollowtrees
```

Configuration of the project is done through a YAML config file. An example for that can be found under `config.yaml.dist`

## Quick architecture overview

>For an introduction and overview of the architecture please read the following blog [post](https://banzaicloud.com/blog/hollowtrees)

![Hollowtrees](docs/images/hollowtrees-overview.png)

## Configuring Prometheus to send alerts to Hollowtrees

Hollowtrees is listening on an API similar to the [Prometheus Alert Manager](https://prometheus.io/docs/alerting/alertmanager/) and it can be configured in Prometheus as an Alert Manager. For example if Hollowtrees is running locally on port 9092 (configurable through `global.bindAddr`), Prometheus can be configured like this to send its alerts to Hollowtrees directly:

```yaml
# Alertmanager configuration
alerting:
  alertmanagers:
  - static_configs:
    - targets:
       - localhost:9092
```

### Configuring action flows

After a Prometheus alert is received by Hollowtrees, it first converts it to an event that complies to the [OpenEvents](https://openevents.io) specification, then it processes it based on the action flows configured in the `config.yaml` file, and sends events to its configured action plugins. An example configuration can be found in `config.yaml.dist` under `plugins` and `flows`.

Hollowtrees uses gRPC to send events to its action plugins, and calls the action plugins sequentially. This very simple rule engine will probably change once Hollowtrees will have a release and will support different calling mechanisms, and passing of configuration parameters to the plugins.

Alerts coming from Prometheus are converted to events with a type of `prometheus.server.alert.<AlertName>`. Prometheus labels are converted to the `data` payload as JSON. Data payload elements can be used in the action flows to forward events to the plugins only when it matches a specific string.

### Advanced control structures in action flows

* `cooldown`: Cooldown time that passes after an action flow is successfully finished. During the cooldown the action flow is considered `in progress`. Format: golang time, e.g.: `5m30s`
* `groupBy`: Categorizes subsequent events as the same, if all the corresponding values of these attributes match
* `filters`: Filter events by event values

### Action plugins

Action plugins are microservices that can react to different Hollowtrees events. They are listening on a gRPC endpoint and processing events in an arbitrary way. An example action plugin is in `examples/grpc_plugin`.

To create an action plugin, the [grpcplugin](github.com/banzaicloud/hollowtrees/pkg/grpcplugin) package must be imported, the `EventHandler` interface must be implemented and the gRPC server must be started with

```go
as.Serve(port, newEventHandler())
```

### License

Copyright (c) 2017-2019 [Banzai Cloud, Inc.](https://banzaicloud.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
