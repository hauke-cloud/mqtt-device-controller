package k8s

import (
	"context"
	"testing"

	iov1 "github.com/hauke-cloud/mqtt-device-controller/api/v1alpha1"
	"github.com/hauke-cloud/mqtt-device-controller/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type mockManager struct {
	renameCalledWith []renameCall
	renameErr        error
}

type renameCall struct {
	bridgeNS, bridgeName, shortAddr, newName string
}

func (m *mockManager) Reconcile(_ context.Context, _ iov1.MQTTBridge) error { return nil }
func (m *mockManager) Remove(_ iov1.MQTTBridge)                             {}
func (m *mockManager) Rename(_ context.Context, bridgeNS, bridgeName, shortAddr, newName string) error {
	m.renameCalledWith = append(m.renameCalledWith, renameCall{bridgeNS, bridgeName, shortAddr, newName})
	return m.renameErr
}

func buildTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := iov1.AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	return s
}

func TestDeviceReconciler_NoRenameWhenNamesMatch(t *testing.T) {
	dev := &iov1.MQTTDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "valve-right",
			Namespace: "iot",
			Annotations: map[string]string{
				iov1.AnnotationBridgeFriendlyName: "valve-right",
			},
		},
		Spec: iov1.MQTTDeviceSpec{
			BridgeRef:    iov1.ObjectRef{Name: "bridge1", Namespace: "iot"},
			FriendlyName: "valve-right",
			ShortAddr:    "0x4F2E",
		},
	}

	k8s := fake.NewClientBuilder().
		WithScheme(buildTestScheme(t)).
		WithObjects(dev).
		Build()

	mock := &mockManager{}
	r := &DeviceReconciler{
		Client:  k8s,
		log:     testLogger(),
		manager: mock,
		metrics: metrics.New(prometheus.NewRegistry()),
	}

	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "valve-right", Namespace: "iot"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.RequeueAfter != 0 {
		t.Errorf("unexpected requeue: %v", res.RequeueAfter)
	}
	if len(mock.renameCalledWith) != 0 {
		t.Errorf("rename should not have been called, got %v", mock.renameCalledWith)
	}
}

func TestDeviceReconciler_RenameWhenNamesDiffer(t *testing.T) {
	dev := &iov1.MQTTDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "valve-right",
			Namespace: "iot",
			Annotations: map[string]string{
				iov1.AnnotationBridgeFriendlyName: "valve-right",
			},
		},
		Spec: iov1.MQTTDeviceSpec{
			BridgeRef:    iov1.ObjectRef{Name: "bridge1", Namespace: "iot"},
			FriendlyName: "valve-right-new",
			ShortAddr:    "0x4F2E",
		},
	}

	k8s := fake.NewClientBuilder().
		WithScheme(buildTestScheme(t)).
		WithObjects(dev).
		Build()

	mock := &mockManager{}
	r := &DeviceReconciler{
		Client:  k8s,
		log:     testLogger(),
		manager: mock,
		metrics: metrics.New(prometheus.NewRegistry()),
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "valve-right", Namespace: "iot"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.renameCalledWith) != 1 {
		t.Fatalf("rename called %d times, want 1", len(mock.renameCalledWith))
	}
	call := mock.renameCalledWith[0]
	if call.newName != "valve-right-new" {
		t.Errorf("newName = %q, want valve-right-new", call.newName)
	}
	if call.shortAddr != "0x4F2E" {
		t.Errorf("shortAddr = %q, want 0x4F2E", call.shortAddr)
	}

	// Verify annotation was updated.
	var updated iov1.MQTTDevice
	if err := k8s.Get(context.Background(), types.NamespacedName{Name: "valve-right", Namespace: "iot"}, &updated); err != nil {
		t.Fatalf("get updated: %v", err)
	}
	if updated.Annotations[iov1.AnnotationBridgeFriendlyName] != "valve-right-new" {
		t.Errorf("annotation = %q, want valve-right-new", updated.Annotations[iov1.AnnotationBridgeFriendlyName])
	}
}

func TestDeviceReconciler_NotFound(t *testing.T) {
	k8s := fake.NewClientBuilder().
		WithScheme(buildTestScheme(t)).
		Build()

	mock := &mockManager{}
	r := &DeviceReconciler{
		Client:  k8s,
		log:     testLogger(),
		manager: mock,
		metrics: metrics.New(prometheus.NewRegistry()),
	}

	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "gone", Namespace: "iot"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != (ctrl.Result{}) {
		t.Errorf("unexpected result: %v", res)
	}
}
