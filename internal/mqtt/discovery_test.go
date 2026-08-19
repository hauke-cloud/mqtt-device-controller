package mqtt

import (
	"context"
	"io"
	"log/slog"
	"testing"

	iov1 "github.com/hauke-cloud/mqtt-device-controller/api/v1alpha1"
	"github.com/hauke-cloud/mqtt-device-controller/internal/device"
	"github.com/hauke-cloud/mqtt-device-controller/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestManager(t *testing.T, objs ...runtime.Object) *Manager {
	t.Helper()
	s := runtime.NewScheme()
	if err := iov1.AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	b := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&iov1.MQTTDevice{})
	for _, o := range objs {
		b = b.WithRuntimeObjects(o)
	}
	return NewManager(
		b.Build(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		metrics.New(prometheus.NewRegistry()),
	)
}

func testBridge(name, bridgeName string) iov1.MQTTBridge {
	return iov1.MQTTBridge{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "iot"},
		Spec:       iov1.MQTTBridgeSpec{BridgeName: bridgeName, Host: "mqtt.example", Port: 1883},
	}
}

// A second coordinator holding a stale entry for a device that lives on another
// bridge must not be able to rewrite spec.friendlyName. Before the ownership
// guard this is what made the CR oscillate between its real name and its short
// address, once per discovery cycle, forever.
func TestUpsertDevice_ForeignBridgeCannotOverwriteFriendlyName(t *testing.T) {
	dev := &iov1.MQTTDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "0xda76",
			Namespace:   "iot",
			Annotations: map[string]string{iov1.AnnotationBridgeFriendlyName: "valve-center"},
		},
		Spec: iov1.MQTTDeviceSpec{
			BridgeRef:    iov1.ObjectRef{Name: "office", Namespace: "iot"},
			FriendlyName: "valve-center",
			IEEEAddr:     "0xF4B3B1FFFE4E213B",
			ShortAddr:    "0xDA76",
		},
	}
	m := newTestManager(t, dev)

	// The bedroom coordinator reports the same short address under a stale,
	// unnamed entry — same physical device, re-paired to office long ago.
	ghost := device.ZbStatus3Item{
		Device:    "0xDA76",
		Name:      "0xDA76",
		IEEEAddr:  "0xF4B3B1FFFE4E213B",
		Reachable: true,
	}
	if err := m.upsertDevice(context.Background(), testBridge("bedroom", "tasmota_bedroom"), ghost); err != nil {
		t.Fatalf("upsertDevice: %v", err)
	}

	var got iov1.MQTTDevice
	if err := m.k8s.Get(context.Background(), types.NamespacedName{Name: "0xda76", Namespace: "iot"}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Spec.FriendlyName != "valve-center" {
		t.Errorf("spec.friendlyName = %q, want %q", got.Spec.FriendlyName, "valve-center")
	}
	if ann := got.Annotations[iov1.AnnotationBridgeFriendlyName]; ann != "valve-center" {
		t.Errorf("bridge-friendly-name annotation = %q, want %q", ann, "valve-center")
	}
	if got.Spec.BridgeRef.Name != "office" {
		t.Errorf("spec.bridgeRef.name = %q, want %q", got.Spec.BridgeRef.Name, "office")
	}
}

// The owning bridge must still be able to sync the name it reports.
func TestUpsertDevice_OwningBridgeStillUpdatesFriendlyName(t *testing.T) {
	dev := &iov1.MQTTDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "0xda76",
			Namespace:   "iot",
			Annotations: map[string]string{iov1.AnnotationBridgeFriendlyName: "old-name"},
		},
		Spec: iov1.MQTTDeviceSpec{
			BridgeRef:    iov1.ObjectRef{Name: "office", Namespace: "iot"},
			FriendlyName: "old-name",
			ShortAddr:    "0xDA76",
		},
	}
	m := newTestManager(t, dev)

	report := device.ZbStatus3Item{
		Device:    "0xDA76",
		Name:      "valve-center",
		IEEEAddr:  "0xF4B3B1FFFE4E213B",
		Reachable: true,
	}
	if err := m.upsertDevice(context.Background(), testBridge("office", "tasmota_office"), report); err != nil {
		t.Fatalf("upsertDevice: %v", err)
	}

	var got iov1.MQTTDevice
	if err := m.k8s.Get(context.Background(), types.NamespacedName{Name: "0xda76", Namespace: "iot"}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Spec.FriendlyName != "valve-center" {
		t.Errorf("spec.friendlyName = %q, want %q", got.Spec.FriendlyName, "valve-center")
	}
}

func TestConnectionEqual(t *testing.T) {
	base := testBridge("office", "tasmota_office")
	base.Spec.Topics = []iov1.TopicConfig{{Topic: "tele/tasmota_office/SENSOR", Type: iov1.TopicTypeTelemetry, QoS: 1}}

	t.Run("status change is not a connection change", func(t *testing.T) {
		other := *base.DeepCopy()
		other.Status.MessagesReceived = 12266497
		other.Status.ConnectionState = iov1.ConnectionStateConnected
		if !connectionEqual(base, other) {
			t.Error("connectionEqual = false for a status-only difference, want true")
		}
	})

	t.Run("default port matches explicit 1883", func(t *testing.T) {
		other := *base.DeepCopy()
		other.Spec.Port = 0
		if !connectionEqual(base, other) {
			t.Error("connectionEqual = false for defaulted port, want true")
		}
	})

	for name, mutate := range map[string]func(*iov1.MQTTBridge){
		"host":             func(b *iov1.MQTTBridge) { b.Spec.Host = "other.example" },
		"port":             func(b *iov1.MQTTBridge) { b.Spec.Port = 8883 },
		"discoveryEnabled": func(b *iov1.MQTTBridge) { b.Spec.DiscoveryEnabled = true },
		"topics":           func(b *iov1.MQTTBridge) { b.Spec.Topics = nil },
	} {
		t.Run(name+" change forces a reconnect", func(t *testing.T) {
			other := *base.DeepCopy()
			mutate(&other)
			if connectionEqual(base, other) {
				t.Errorf("connectionEqual = true after %s change, want false", name)
			}
		})
	}
}
