<!-- llm-readme-management spec=1 commit=1f400474a6128da2d36b18f5814994f9212914bc template=golang model=qwen3.8-27b-q4 digest=b3d4b07f2e19 generated=2026-09-30T13:29:03Z -->
<a href="https://hauke.cloud" target="_blank"><img src="https://img.shields.io/badge/home-hauke.cloud-brightgreen" alt="hauke.cloud" style="display: block;" /></a>
<a href="https://github.com/hauke-cloud" target="_blank"><img src="https://img.shields.io/badge/github-hauke.cloud-blue" alt="hauke.cloud Github Organisation" style="display: block;" /></a>
<a href="https://github.com/hauke-cloud/llm-readme-management" target="_blank"><img src="https://img.shields.io/badge/template-golang-orange" alt="Repository type - golang" style="display: block;" /></a>


# Mqtt Device Controller


<img src="https://raw.githubusercontent.com/hauke-cloud/.github/main/resources/img/organisation-logo-small.png" alt="hauke.cloud logo" width="109" height="123" align="right">


<llm header hint="Name the Go module path and say whether this is a service, a CLI or a library.">

This is a Go service (module `github.com/hauke-cloud/mqtt-device-controller`) that runs as a Kubernetes controller. It discovers Zigbee devices behind Tasmota MQTT bridges, creates and updates `MQTTDevice` resources in the `iot.hauke.cloud` API group, pushes rename commands to the bridge, and serves an mTLS REST API with Prometheus metrics. It is for you if you run a Kubernetes IoT stack with Tasmota bridges.

</llm>


## :book: Description

<llm description>

`mqtt-device-controller` is a Kubernetes controller that discovers Zigbee devices behind Tasmota MQTT bridges and represents them as `MQTTDevice` custom resources in the `iot.hauke.cloud/v1alpha1` API group. It watches `MQTTBridge` CRs (managed by a separate operator in the `hauke-cloud` stack) and, for each bridge with discovery enabled, runs a 30-second loop that queries the Tasmota firmware over MQTT for the device list and per-device details, then upserts one CR per device carrying its IEEE address, short address, friendly name, model, battery, link quality, and reachability.

It also watches `MQTTDevice` CRs: when an operator edits `spec.friendlyName`, the controller publishes a rename command back to the bridge. A mutual-TLS REST API and Prometheus metrics are exposed for querying and monitoring.

- Discovers Zigbee devices via a 30-second MQTT polling loop and upserts `MQTTDevice` CRs keyed by short address.
- Publishes a `zbname` rename command to the bridge when `spec.friendlyName` changes.
- Serves a REST API (`GET /api/v1/devices`, `GET /api/v1/devices/{identifier}`) behind mandatory mTLS on port 8443.
- Registers Prometheus metrics (device counts, reachability, battery, discovery duration, bridge connectivity) under the `mqtt_device_controller` namespace.
- Ships a Helm chart with optional PrometheusRule alerts and a Grafana dashboard ConfigMap.

</llm>


## :clipboard: Requirements

<llm requirements hint="Give the Go version from the go directive in go.mod. Mention Docker only if the repository actually builds an image.">

- Go 1.24 (the `go` directive in `go.mod`) for building from source.
- Docker, for building the container image (`make docker-build`).
- A Kubernetes cluster the controller can reach (in-cluster service account or kubeconfig); the Helm chart creates the required ServiceAccount and Role.
- A running MQTT broker (Tasmota firmware) for each `MQTTBridge` CR, reachable at the configured `host:port`.
- TLS material for the mTLS REST API: a pre-existing Kubernetes Secret with keys `tls.crt`, `tls.key`, and `ca.crt`, or cert-manager installed in the cluster with a suitable Issuer.
- `pre-commit` (hooks v4.4.0) for running the repository's pre-commit hooks.
- `controller-gen` v0.17.0 and `setup-envtest` (k8s 1.31.0) for regenerating CRD/RBAC manifests and running tests.

</llm>


## 🚀 Getting started

<llm getting_started hint="Cover go build, go run and go test with the real package paths. If a Makefile or Taskfile exists, prefer its targets over raw go commands.">

1. Clone the repository.

```bash
git clone https://github.com/hauke-cloud/mqtt-device-controller.git
cd mqtt-device-controller
```

2. Build the controller binary; this target also runs `gofmt` and `go vet` before compiling.

```bash
make build
```

3. Run the test suite with race detection and coverage reporting.

```bash
make test
```

After step 2 you have a working binary at `bin/controller`. After step 3 you have confirmed that the reconcilers, MQTT discovery logic, REST handlers, and payload parsers all pass their unit tests. To deploy the controller into a cluster you would use the Helm chart under `deployments/helm/mqtt-device-controller/`, but that requires a running Kubernetes cluster, at least one `MQTTBridge` CR pointing at a Tasmota bridge, and TLS certificates for the mTLS API.

</llm>


## :airplane: Usage

<llm usage hint="For a library, show a small import-and-call example using real exported identifiers. For a service or CLI, show how it is started and the flags or subcommands it accepts.">

Once the controller is running in your cluster, you work with it through the Helm chart, `MQTTBridge` custom resources, and the mTLS REST API.

**Deploy the chart.** The chart lives in `deployments/helm/mqtt-device-controller/`. The REST API will not start without a TLS secret (or cert-manager) providing `tls.crt`, `tls.key`, and `ca.crt`:

```yaml
# values.yaml
tls:
  existingSecret: mqtt-dc-tls
watchNamespace: ""
controller:
  leaderElect: true
crds:
  install: true
```

```bash
helm install mqtt-device-controller deployments/helm/mqtt-device-controller -f values.yaml
```

**Point the controller at a bridge.** A separate operator manages `MQTTBridge` resources; you create one per Tasmota bridge. Discovery runs only when `discoveryEnabled` is `true`:

```yaml
apiVersion: iot.hauke.cloud/v1alpha1
kind: MQTTBridge
metadata:
  name: living-room
  namespace: iot
spec:
  bridgeName: living-room
  host: tasmota-bridge
  port: 1883
  deviceType: tasmota
  discoveryEnabled: true
  maxReconnectBackoffSeconds: 60
  credentialsSecretRef:
    name: tasmota-creds
    namespace: iot
```

**Query devices over the REST API.** The API on port 8443 enforces mutual TLS (TLS 1.3, client certificate required). List devices with pagination, or look one up by CR name, friendly name, short address (`0x…`), or IEEE address:

```bash
curl --cacert ca.crt --cert client.crt --key client.key \
  "https://mqtt-device-controller.iot.svc.cluster.local:8443/api/v1/devices?limit=20&offset=0"
```

To rename a device, edit `spec.friendlyName` on the corresponding `MQTTDevice` CR; the controller publishes the rename to the bridge on the next reconcile.

</llm>


## :wrench: Configuration

<llm configuration hint="Environment variables and CLI flags, taken from the flag definitions or the config struct.">

The `/controller` binary accepts the following flags (defined in `cmd/controller/main.go`):

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--metrics-bind-address` | string | `:8080` | controller-runtime metrics endpoint |
| `--health-probe-bind-address` | string | `:8081` | `/healthz` and `/readyz` probes |
| `--api-bind-address` | string | `:8443` | mTLS REST API listener |
| `--tls-cert-file` | string | `/tls/tls.crt` | server certificate for the API |
| `--tls-key-file` | string | `/tls/tls.key` | server key for the API |
| `--tls-ca-file` | string | `/tls/ca.crt` | CA used to verify client certificates |
| `--namespace` | string | `""` (all) | scope for REST API device queries |
| `--leader-elect` | bool | `false` | enable leader election (Helm sets `true`) |

The Helm chart in `deployments/helm/mqtt-device-controller/values.yaml` maps most of these flags and adds image settings, `watchNamespace`, TLS via `tls.existingSecret` or `certManager`, service and resource limits, and optional `serviceMonitor`, `prometheusRule`, and `grafanaDashboard` toggles. See that file for the full set.

Operators also configure two CRDs: `MQTTBridge` (fields include `bridgeName`, `host`, `port`, `deviceType`, `discoveryEnabled`, `credentialsSecretRef`, `topics`) and `MQTTDevice` (fields include `bridgeRef`, `friendlyName`, `ieeeAddr`, `shortAddr`, `disabled`). The complete field definitions live in `api/v1alpha1/mqttbridge_types.go` and `api/v1alpha1/mqttdevice_types.go`.

</llm>


## :hammer: Development

<llm development hint="Include go test, go vet and gofmt only where the CI workflows actually run them.">

Install the pre-commit hooks before your first commit:

```bash
pre-commit install
```

They check for trailing whitespace, large files, merge-conflict markers, VCS permalinks, private keys, AWS credentials, missing EOF newlines, and commits to protected branches. `gitleaks` v8.18.0 also scans for leaked secrets. Run all hooks manually with:

```bash
pre-commit run --all-files
```

**Tests.** CI runs the same command as the Makefile target:

```bash
make test
```

This downloads envtest assets (k8s 1.31.0) and executes `go test -v -race -coverprofile=coverage.out -covermode=atomic ./...`.

**Lint and format.** CI enforces:

```bash
gofmt -s -l . && go vet ./...
```

The equivalent Makefile target is `make lint`; `make build` runs the same checks before compiling.

**Generated files.** If you change types under `api/`, regenerate and commit the output:

```bash
make generate
make manifests
```

CI runs both before the build step; a stale `zz_generated.deepcopy.go` or `config/crd/bases/` file will fail the pipeline.

**PR title.** A semantic-pull-request check validates that your PR title follows Conventional Commits (e.g. `feat: …`, `fix: …`). Non-conforming titles are rejected.

</llm>


## 📄 License

This Project is licensed under the GNU General Public License v3.0

- see the [LICENSE](LICENSE) file for details.


## :coffee: Contributing

To become a contributor, please check out the [CONTRIBUTING](CONTRIBUTING.md) file.


## :email: Contact

For any inquiries or support requests, please open an issue in this
repository or contact us at [contact@hauke.cloud](mailto:contact@hauke.cloud).
