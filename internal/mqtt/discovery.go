package mqtt

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	iov1 "github.com/hauke-cloud/mqtt-device-controller/api/v1alpha1"
	"github.com/hauke-cloud/mqtt-device-controller/internal/device"
	"github.com/hauke-cloud/mqtt-device-controller/internal/metrics"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	discoveryInterval = 30 * time.Second
	zbStatus1Timeout  = 15 * time.Second
	zbStatus3Timeout  = 10 * time.Second
)

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9-]+`)

// Manager maintains one BridgeClient per active MQTTBridge and runs discovery loops.
type Manager struct {
	mu      sync.RWMutex
	clients map[string]*BridgeClient
	cancels map[string]context.CancelFunc

	k8s     client.Client
	log     *slog.Logger
	metrics *metrics.Metrics
}

// NewManager constructs a Manager.
func NewManager(k8s client.Client, log *slog.Logger, m *metrics.Metrics) *Manager {
	return &Manager{
		clients: make(map[string]*BridgeClient),
		cancels: make(map[string]context.CancelFunc),
		k8s:     k8s,
		log:     log,
		metrics: m,
	}
}

// Reconcile is called by the BridgeReconciler whenever a MQTTBridge changes.
func (m *Manager) Reconcile(ctx context.Context, bridge iov1.MQTTBridge) error {
	key := bridgeKey(bridge)

	if !bridge.Spec.DiscoveryEnabled || bridge.DeletionTimestamp != nil {
		m.remove(key)
		return nil
	}

	username, password, err := m.resolveCredentials(ctx, bridge)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.clients[key]; ok {
		existing.Disconnect()
		if cancel, ok := m.cancels[key]; ok {
			cancel()
		}
		delete(m.clients, key)
		delete(m.cancels, key)
	}

	bc, err := newBridgeClient(ctx, bridge, username, password, m.log, m.metrics)
	if err != nil {
		return fmt.Errorf("bridge %q: %w", bridge.Spec.BridgeName, err)
	}

	loopCtx, cancel := context.WithCancel(context.Background())
	m.clients[key] = bc
	m.cancels[key] = cancel

	go m.discoveryLoop(loopCtx, bc, bridge)
	return nil
}

// Remove tears down the client for a bridge that was deleted or disabled.
func (m *Manager) Remove(bridge iov1.MQTTBridge) {
	m.remove(bridgeKey(bridge))
}

// Rename sends a zbname rename command via the bridge that owns the device.
func (m *Manager) Rename(ctx context.Context, bridgeNS, bridgeName, shortAddr, newName string) error {
	key := fmt.Sprintf("%s/%s", bridgeNS, bridgeName)
	m.mu.RLock()
	bc, ok := m.clients[key]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("no active MQTT client for bridge %s", key)
	}
	return bc.SendRename(ctx, shortAddr, newName)
}

func (m *Manager) remove(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cancel, ok := m.cancels[key]; ok {
		cancel()
		delete(m.cancels, key)
	}
	if bc, ok := m.clients[key]; ok {
		bc.Disconnect()
		delete(m.clients, key)
	}
}

func (m *Manager) discoveryLoop(ctx context.Context, bc *BridgeClient, bridge iov1.MQTTBridge) {
	log := m.log.With("bridge", bridge.Spec.BridgeName)
	ticker := time.NewTicker(discoveryInterval)
	defer ticker.Stop()

	m.runDiscovery(ctx, bc, bridge)

	for {
		select {
		case <-ticker.C:
			m.runDiscovery(ctx, bc, bridge)
		case <-ctx.Done():
			log.Info("discovery loop stopped")
			return
		}
	}
}

func (m *Manager) runDiscovery(ctx context.Context, bc *BridgeClient, bridge iov1.MQTTBridge) {
	log := m.log.With("bridge", bridge.Spec.BridgeName)
	start := time.Now()

	items, err := bc.SendZbStatus1(ctx, zbStatus1Timeout)
	if err != nil {
		log.Warn("ZbStatus1 failed", "err", err)
		m.metrics.DiscoveryRunsTotal.WithLabelValues(bridge.Spec.BridgeName, "error").Inc()
		return
	}

	m.metrics.DevicesTotal.WithLabelValues(bridge.Spec.BridgeName).Set(float64(len(items)))

	// Keyed by short address (e.g. "0x4F2E") — stable across friendly-name renames.
	seenShortAddrs := make(map[string]bool, len(items))
	reachableCount := 0

	for _, item := range items {
		if item.Device == "" {
			continue
		}
		seenShortAddrs[item.Device] = true

		details, err := bc.SendZbStatus3(ctx, item.Device, zbStatus3Timeout)
		if err != nil {
			log.Warn("ZbStatus3 failed", "device", item.Device, "err", err)
			continue
		}
		if details.Reachable {
			reachableCount++
		}
		if upsertErr := m.upsertDevice(ctx, bridge, details); upsertErr != nil {
			log.Error("upsert device failed", "device", item.Name, "err", upsertErr)
		}

		m.metrics.DeviceLastSeenAge.WithLabelValues(item.Name, bridge.Spec.BridgeName).Set(0)
		if details.BatteryPercentage > 0 {
			m.metrics.DeviceBattery.WithLabelValues(item.Name, bridge.Spec.BridgeName).
				Set(float64(details.BatteryPercentage))
		}
	}

	m.metrics.DevicesReachable.WithLabelValues(bridge.Spec.BridgeName).Set(float64(reachableCount))

	if err := m.markMissingDevicesStale(ctx, bridge, seenShortAddrs); err != nil {
		log.Warn("mark stale devices failed", "err", err)
	}

	m.metrics.DiscoveryRunsTotal.WithLabelValues(bridge.Spec.BridgeName, "success").Inc()
	m.metrics.DiscoveryDuration.WithLabelValues(bridge.Spec.BridgeName).Observe(time.Since(start).Seconds())
}

func (m *Manager) upsertDevice(ctx context.Context, bridge iov1.MQTTBridge, d device.ZbStatus3Item) error {
	// Use the short address as the stable CRD name so a friendly-name rename
	// does not cause a second CR to be created.
	name := sanitizeName(d.Device)
	namespace := bridge.Namespace
	now := metav1.NewTime(time.Now())

	reachable := d.Reachable
	battery := int32(d.BatteryPercentage)
	lq := int32(d.LinkQuality)
	endpoints := make([]int32, len(d.Endpoints))
	for i, e := range d.Endpoints {
		endpoints[i] = int32(e)
	}

	var existing iov1.MQTTDevice
	nsName := types.NamespacedName{Name: name, Namespace: namespace}
	err := m.k8s.Get(ctx, nsName, &existing)

	if k8serrors.IsNotFound(err) {
		dev := iov1.MQTTDevice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Annotations: map[string]string{
					iov1.AnnotationBridgeFriendlyName: d.Name,
				},
			},
			Spec: iov1.MQTTDeviceSpec{
				BridgeRef:    iov1.ObjectRef{Name: bridge.Name, Namespace: bridge.Namespace},
				FriendlyName: d.Name,
				IEEEAddr:     d.IEEEAddr,
				ShortAddr:    d.Device,
			},
		}
		controllerutil.AddFinalizer(&dev, "iot.hauke.cloud/finalizer")
		applyStatus(&dev, now, d.ModelID, d.Manufacturer, reachable, battery, lq, endpoints)
		return m.k8s.Create(ctx, &dev)
	}
	if err != nil {
		return fmt.Errorf("get MQTTDevice %s: %w", name, err)
	}

	// Patch spec fields owned by the controller.
	patch := client.MergeFrom(existing.DeepCopy())
	if existing.Spec.IEEEAddr == "" {
		existing.Spec.IEEEAddr = d.IEEEAddr
	}
	if existing.Spec.ShortAddr == "" {
		existing.Spec.ShortAddr = d.Device
	}
	// Keep spec.friendlyName in sync with whatever the bridge currently reports,
	// but only when the DeviceReconciler is not mid-rename (annotation == spec means
	// a confirmed rename is complete; annotation != spec means rename is in flight).
	if existing.Annotations == nil {
		existing.Annotations = make(map[string]string)
	}
	bridgeSideName := existing.Annotations[iov1.AnnotationBridgeFriendlyName]
	if bridgeSideName == existing.Spec.FriendlyName {
		// No rename in flight — safe to update both spec and annotation.
		existing.Spec.FriendlyName = d.Name
		existing.Annotations[iov1.AnnotationBridgeFriendlyName] = d.Name
	} else {
		// Rename in flight (DeviceReconciler hasn't confirmed yet) —
		// only update the annotation so the reconciler can detect completion.
		existing.Annotations[iov1.AnnotationBridgeFriendlyName] = d.Name
	}
	if err := m.k8s.Patch(ctx, &existing, patch); err != nil {
		return fmt.Errorf("patch MQTTDevice %s: %w", name, err)
	}

	// Update status.
	applyStatus(&existing, now, d.ModelID, d.Manufacturer, reachable, battery, lq, endpoints)
	return m.k8s.Status().Update(ctx, &existing)
}

func applyStatus(dev *iov1.MQTTDevice, now metav1.Time, modelID, manufacturer string, reachable bool, battery, lq int32, endpoints []int32) {
	dev.Status.LastSeenTime = &now
	dev.Status.LastUpdatedTime = &now
	dev.Status.ModelID = modelID
	dev.Status.Manufacturer = manufacturer
	dev.Status.Reachable = &reachable
	dev.Status.BatteryPct = &battery
	dev.Status.LinkQuality = &lq
	dev.Status.Endpoints = endpoints

	readyStatus := metav1.ConditionTrue
	readyReason := iov1.ReasonReachable
	readyMsg := "Device is reachable"
	if !reachable {
		readyStatus = metav1.ConditionFalse
		readyReason = iov1.ReasonUnreachable
		readyMsg = "Device is not reachable"
	}
	k8smeta.SetStatusCondition(&dev.Status.Conditions, metav1.Condition{
		Type:               iov1.ConditionTypeReady,
		Status:             readyStatus,
		Reason:             readyReason,
		Message:            readyMsg,
		ObservedGeneration: dev.Generation,
	})
	k8smeta.SetStatusCondition(&dev.Status.Conditions, metav1.Condition{
		Type:               iov1.ConditionTypeDiscovered,
		Status:             metav1.ConditionTrue,
		Reason:             iov1.ReasonDiscovered,
		Message:            "Device was seen in the last discovery cycle",
		ObservedGeneration: dev.Generation,
	})
}

func (m *Manager) markMissingDevicesStale(ctx context.Context, bridge iov1.MQTTBridge, seen map[string]bool) error {
	var list iov1.MQTTDeviceList
	if err := m.k8s.List(ctx, &list,
		client.InNamespace(bridge.Namespace),
		client.MatchingFields{"spec.bridgeRef.name": bridge.Name},
	); err != nil {
		return fmt.Errorf("list devices for bridge %s: %w", bridge.Name, err)
	}

	for i := range list.Items {
		dev := &list.Items[i]
		if seen[dev.Spec.ShortAddr] {
			continue
		}
		k8smeta.SetStatusCondition(&dev.Status.Conditions, metav1.Condition{
			Type:               iov1.ConditionTypeDiscovered,
			Status:             metav1.ConditionFalse,
			Reason:             iov1.ReasonNotDiscovered,
			Message:            "Device was not found in the last discovery cycle",
			ObservedGeneration: dev.Generation,
		})
		k8smeta.SetStatusCondition(&dev.Status.Conditions, metav1.Condition{
			Type:               iov1.ConditionTypeReady,
			Status:             metav1.ConditionFalse,
			Reason:             iov1.ReasonNotDiscovered,
			Message:            "Device was not found in the last discovery cycle",
			ObservedGeneration: dev.Generation,
		})
		if err := m.k8s.Status().Update(ctx, dev); err != nil {
			m.log.Warn("mark device stale failed", "device", dev.Name, "err", err)
		}
	}
	return nil
}

func (m *Manager) resolveCredentials(ctx context.Context, bridge iov1.MQTTBridge) (string, string, error) {
	ref := bridge.Spec.CredentialsSecretRef
	if ref == nil {
		return "", "", nil
	}

	var secret corev1.Secret
	if err := m.k8s.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}, &secret); err != nil {
		return "", "", fmt.Errorf("get credentials secret %s/%s: %w", ref.Namespace, ref.Name, err)
	}

	usernameKey := "username"
	if ref.UsernameKey != "" {
		usernameKey = ref.UsernameKey
	}
	passwordKey := "password"
	if ref.PasswordKey != "" {
		passwordKey = ref.PasswordKey
	}
	return string(secret.Data[usernameKey]), string(secret.Data[passwordKey]), nil
}

func bridgeKey(bridge iov1.MQTTBridge) string {
	return fmt.Sprintf("%s/%s", bridge.Namespace, bridge.Name)
}

func sanitizeName(name string) string {
	s := strings.ToLower(name)
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 63 {
		s = s[:63]
	}
	return s
}
