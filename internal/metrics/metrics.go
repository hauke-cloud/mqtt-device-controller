package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const namespace = "mqtt_device_controller"

type Metrics struct {
	DevicesTotal        *prometheus.GaugeVec
	DevicesReachable    *prometheus.GaugeVec
	DiscoveryRunsTotal  *prometheus.CounterVec
	DiscoveryDuration   *prometheus.HistogramVec
	DeviceLastSeenAge   *prometheus.GaugeVec
	DeviceBattery       *prometheus.GaugeVec
	BridgeConnected     *prometheus.GaugeVec
	MessagesReceived    *prometheus.CounterVec
	RenameTotal         *prometheus.CounterVec
}

func New(reg prometheus.Registerer) *Metrics {
	factory := promauto.With(reg)
	return &Metrics{
		DevicesTotal: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "devices_total",
			Help:      "Total number of Zigbee devices known per bridge.",
		}, []string{"bridge"}),

		DevicesReachable: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "devices_reachable",
			Help:      "Number of reachable Zigbee devices per bridge.",
		}, []string{"bridge"}),

		DiscoveryRunsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "discovery_runs_total",
			Help:      "Total number of discovery cycles per bridge.",
		}, []string{"bridge", "status"}),

		DiscoveryDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "discovery_duration_seconds",
			Help:      "Duration of a full discovery cycle per bridge.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"bridge"}),

		DeviceLastSeenAge: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "device_last_seen_seconds",
			Help:      "Seconds since the device was last seen by the bridge.",
		}, []string{"device", "bridge"}),

		DeviceBattery: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "device_battery_percentage",
			Help:      "Battery percentage reported by the device.",
		}, []string{"device", "bridge"}),

		BridgeConnected: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "bridge_connected",
			Help:      "1 if the MQTT connection to the bridge is active, 0 otherwise.",
		}, []string{"bridge"}),

		MessagesReceived: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "mqtt_messages_received_total",
			Help:      "Total MQTT messages received per bridge and topic type.",
		}, []string{"bridge", "topic_type"}),

		RenameTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "device_renames_total",
			Help:      "Total rename commands issued per bridge.",
		}, []string{"bridge", "status"}),
	}
}
