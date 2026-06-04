package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	iov1 "github.com/hauke-cloud/mqtt-device-controller/api/v1alpha1"
	"github.com/go-chi/chi/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func buildScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := iov1.AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	return s
}

func makeDevice(name, ns, friendlyName, ieeeAddr, shortAddr, bridge string) iov1.MQTTDevice {
	reachable := true
	battery := int32(80)
	lq := int32(100)
	return iov1.MQTTDevice{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: iov1.MQTTDeviceSpec{
			FriendlyName: friendlyName,
			IEEEAddr:     ieeeAddr,
			ShortAddr:    shortAddr,
			BridgeRef:    iov1.ObjectRef{Name: bridge, Namespace: ns},
		},
		Status: iov1.MQTTDeviceStatus{
			ModelID:     "lumi.weather",
			Reachable:   &reachable,
			BatteryPct:  &battery,
			LinkQuality: &lq,
		},
	}
}

func TestListDevices(t *testing.T) {
	ns := "iot"
	devices := []runtime.Object{
		func() runtime.Object { d := makeDevice("room-floor3", ns, "room-floor3", "0xABCD", "0x3A65", "bridge1"); return &d }(),
		func() runtime.Object { d := makeDevice("valve-right", ns, "valve-right", "0x1234", "0x4F2E", "bridge1"); return &d }(),
	}
	k8s := fake.NewClientBuilder().
		WithScheme(buildScheme(t)).
		WithRuntimeObjects(devices...).
		Build()

	h := NewHandler(k8s, ns)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	rr := httptest.NewRecorder()
	h.ListDevices(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var envelope collectionEnvelope
	if err := json.NewDecoder(rr.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Total != 2 {
		t.Errorf("total = %d, want 2", envelope.Total)
	}
}

func TestListDevicesPagination(t *testing.T) {
	ns := "iot"
	devices := []runtime.Object{}
	for i := 0; i < 5; i++ {
		name := "device-" + string(rune('a'+i))
		d := makeDevice(name, ns, name, "0x000"+string(rune('0'+i)), "0x000"+string(rune('0'+i)), "bridge1")
		devices = append(devices, &d)
	}
	k8s := fake.NewClientBuilder().
		WithScheme(buildScheme(t)).
		WithRuntimeObjects(devices...).
		Build()

	h := NewHandler(k8s, ns)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices?limit=2&offset=0", nil)
	rr := httptest.NewRecorder()
	h.ListDevices(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var envelope collectionEnvelope
	if err := json.NewDecoder(rr.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Total != 5 {
		t.Errorf("total = %d, want 5", envelope.Total)
	}
	if envelope.Limit != 2 {
		t.Errorf("limit = %d, want 2", envelope.Limit)
	}
}

func TestGetDeviceByName(t *testing.T) {
	ns := "iot"
	d := makeDevice("room-floor3", ns, "room-floor3", "0xABCDEF", "0x3A65", "bridge1")
	k8s := fake.NewClientBuilder().
		WithScheme(buildScheme(t)).
		WithRuntimeObjects(&d).
		Build()

	h := NewHandler(k8s, ns)

	r := chi.NewRouter()
	r.Get("/api/v1/devices/{identifier}", h.GetDevice)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/room-floor3", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var view DeviceView
	if err := json.NewDecoder(rr.Body).Decode(&view); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if view.FriendlyName != "room-floor3" {
		t.Errorf("friendlyName = %q, want room-floor3", view.FriendlyName)
	}
}

func TestGetDeviceNotFound(t *testing.T) {
	ns := "iot"
	k8s := fake.NewClientBuilder().
		WithScheme(buildScheme(t)).
		Build()

	h := NewHandler(k8s, ns)

	r := chi.NewRouter()
	r.Get("/api/v1/devices/{identifier}", h.GetDevice)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/nonexistent", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestGetDeviceByIEEEAddr(t *testing.T) {
	ns := "iot"
	d := makeDevice("room-floor3", ns, "room-floor3", "0x00158D008C7CCC7B", "0x3A65", "bridge1")
	k8s := fake.NewClientBuilder().
		WithScheme(buildScheme(t)).
		WithRuntimeObjects(&d).
		Build()

	h := NewHandler(k8s, ns)

	r := chi.NewRouter()
	r.Get("/api/v1/devices/{identifier}", h.GetDevice)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/0x00158D008C7CCC7B", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestParsePagination_InvalidLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?limit=9999", nil)
	_, _, err := parsePagination(req)
	if err == nil {
		t.Error("expected error for limit > 100")
	}
}

func TestToView(t *testing.T) {
	d := makeDevice("test", "ns", "test-device", "0xABCD", "0x1234", "bridge1")
	v := toView(d)
	if v.Name != "test" {
		t.Errorf("Name = %q", v.Name)
	}
	if v.FriendlyName != "test-device" {
		t.Errorf("FriendlyName = %q", v.FriendlyName)
	}
	if v.Bridge != "bridge1" {
		t.Errorf("Bridge = %q", v.Bridge)
	}
	if v.ModelID != "lumi.weather" {
		t.Errorf("ModelID = %q", v.ModelID)
	}
}

// Ensure collectionEnvelope Items can be decoded as a slice when testing.
func (c *collectionEnvelope) UnmarshalJSON(data []byte) error {
	var raw struct {
		Items  json.RawMessage `json:"items"`
		Total  int             `json:"total"`
		Limit  int             `json:"limit"`
		Offset int             `json:"offset"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.Total = raw.Total
	c.Limit = raw.Limit
	c.Offset = raw.Offset
	c.Items = raw.Items
	return nil
}

var _ context.Context = context.Background()
