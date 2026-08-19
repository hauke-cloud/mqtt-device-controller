package k8s

import (
	"context"
	"fmt"
	"log/slog"

	iov1 "github.com/hauke-cloud/mqtt-device-controller/api/v1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// BridgeReconciler watches MQTTBridge CRDs and keeps the MQTT Manager in sync.
type BridgeReconciler struct {
	client.Client
	log     *slog.Logger
	manager DiscoveryManager
}

func NewBridgeReconciler(c client.Client, log *slog.Logger, mgr DiscoveryManager) *BridgeReconciler {
	return &BridgeReconciler{Client: c, log: log, manager: mgr}
}

func (r *BridgeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.log.With("bridge", req.NamespacedName)

	var bridge iov1.MQTTBridge
	if err := r.Get(ctx, req.NamespacedName, &bridge); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get MQTTBridge: %w", err)
	}

	if err := r.manager.Reconcile(ctx, bridge); err != nil {
		log.Warn("manager reconcile failed", "err", err)
		// Requeue with backoff so transient MQTT connect failures are retried.
		return ctrl.Result{RequeueAfter: retryBackoff}, nil
	}

	return ctrl.Result{}, nil
}

func (r *BridgeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Only react to spec changes. MQTTBridge.status is written continuously by
	// the bridge operator (messagesReceived alone ticks several times a
	// minute), and without this predicate every one of those writes reaches
	// Manager.Reconcile and recycles the MQTT connection.
	return ctrl.NewControllerManagedBy(mgr).
		For(&iov1.MQTTBridge{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		Complete(r)
}
