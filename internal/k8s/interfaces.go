package k8s

import (
	"context"

	iov1 "github.com/hauke-cloud/mqtt-device-controller/api/v1alpha1"
)

// DiscoveryManager is the interface that both the real mqtt.Manager and test mocks implement.
type DiscoveryManager interface {
	Reconcile(ctx context.Context, bridge iov1.MQTTBridge) error
	Remove(bridge iov1.MQTTBridge)
	Rename(ctx context.Context, bridgeNS, bridgeName, shortAddr, newName string) error
}
