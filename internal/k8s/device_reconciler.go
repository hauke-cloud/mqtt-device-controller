package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	iov1 "github.com/hauke-cloud/mqtt-device-controller/api/v1alpha1"
	"github.com/hauke-cloud/mqtt-device-controller/internal/metrics"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const retryBackoff = 30 * time.Second

// DeviceReconciler watches MQTTDevice CRDs and issues rename commands when
// spec.friendlyName differs from the bridge-side name stored in the annotation.
type DeviceReconciler struct {
	client.Client
	log     *slog.Logger
	manager DiscoveryManager
	metrics *metrics.Metrics
}

func NewDeviceReconciler(c client.Client, log *slog.Logger, mgr DiscoveryManager, m *metrics.Metrics) *DeviceReconciler {
	return &DeviceReconciler{Client: c, log: log, manager: mgr, metrics: m}
}

func (r *DeviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.log.With("device", req.NamespacedName)

	var dev iov1.MQTTDevice
	if err := r.Get(ctx, req.NamespacedName, &dev); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get MQTTDevice: %w", err)
	}

	bridgeName := dev.Spec.BridgeRef.Name
	if dev.Spec.FriendlyName == "" || bridgeName == "" || dev.Spec.ShortAddr == "" {
		return ctrl.Result{}, nil
	}

	bridgeSideName := dev.Annotations[iov1.AnnotationBridgeFriendlyName]
	if bridgeSideName == dev.Spec.FriendlyName {
		return ctrl.Result{}, nil
	}

	log.Info("friendly name change detected, sending rename",
		"from", bridgeSideName, "to", dev.Spec.FriendlyName)

	err := r.manager.Rename(ctx, dev.Spec.BridgeRef.Namespace, bridgeName, dev.Spec.ShortAddr, dev.Spec.FriendlyName)
	if err != nil {
		log.Warn("rename command failed", "err", err)
		r.metrics.RenameTotal.WithLabelValues(bridgeName, "error").Inc()
		return ctrl.Result{RequeueAfter: retryBackoff}, nil
	}

	r.metrics.RenameTotal.WithLabelValues(bridgeName, "success").Inc()

	// Update the annotation to reflect the new bridge-side name.
	patch := client.MergeFrom(dev.DeepCopy())
	if dev.Annotations == nil {
		dev.Annotations = make(map[string]string)
	}
	dev.Annotations[iov1.AnnotationBridgeFriendlyName] = dev.Spec.FriendlyName
	if err := r.Patch(ctx, &dev, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch annotation after rename: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *DeviceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&iov1.MQTTDevice{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		WithOptions(controllerOptions()).
		Complete(r)
}

func controllerOptions() controller.Options {
	return controller.Options{MaxConcurrentReconciles: 5}
}
