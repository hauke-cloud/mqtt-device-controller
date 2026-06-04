package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// AnnotationBridgeFriendlyName tracks the last name confirmed on the bridge,
	// used to detect user-initiated renames (spec.friendlyName differs from this annotation).
	AnnotationBridgeFriendlyName = "iot.hauke.cloud/bridge-friendly-name"

	ConditionTypeReady      = "Ready"
	ConditionTypeDiscovered = "Discovered"

	ReasonDiscovered    = "Discovered"
	ReasonNotDiscovered = "NotDiscovered"
	ReasonReachable     = "Reachable"
	ReasonUnreachable   = "Unreachable"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=md
// +kubebuilder:printcolumn:name="FriendlyName",type="string",JSONPath=".spec.friendlyName"
// +kubebuilder:printcolumn:name="Bridge",type="string",JSONPath=".spec.bridgeRef.name"
// +kubebuilder:printcolumn:name="Model",type="string",JSONPath=".status.modelId"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type MQTTDevice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MQTTDeviceSpec   `json:"spec"`
	Status MQTTDeviceStatus `json:"status,omitempty"`
}

type MQTTDeviceSpec struct {
	// BridgeRef identifies the MQTTBridge that reported this device.
	// +kubebuilder:validation:Required
	BridgeRef ObjectRef `json:"bridgeRef"`

	// Disabled prevents the device from being polled or commanded.
	// +kubebuilder:default=false
	Disabled bool `json:"disabled,omitempty"`

	// FriendlyName is the human-readable name of the device on the Tasmota bridge.
	// Changing this field triggers a rename command to the bridge.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	FriendlyName string `json:"friendlyName"`

	// IEEEAddr is the long (64-bit) IEEE address of the Zigbee device.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^0x[0-9A-Fa-f]+$`
	IEEEAddr string `json:"ieeeAddr"`

	// ShortAddr is the 16-bit network address assigned by the coordinator.
	// +kubebuilder:validation:Pattern=`^0x[0-9A-Fa-f]{4}$`
	ShortAddr string `json:"shortAddr,omitempty"`

	// TypeRef optionally links this device to a type-specific controller CR.
	TypeRef *TypeRef `json:"typeRef,omitempty"`
}

type ObjectRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type TypeRef struct {
	// +kubebuilder:validation:Required
	APIGroup string `json:"apiGroup"`
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

type MQTTDeviceStatus struct {
	Conditions      []metav1.Condition `json:"conditions,omitempty"`
	LastSeenTime    *metav1.Time       `json:"lastSeenTime,omitempty"`
	LastUpdatedTime *metav1.Time       `json:"lastUpdatedTime,omitempty"`
	ModelID         string             `json:"modelId,omitempty"`
	Manufacturer    string             `json:"manufacturer,omitempty"`
	Reachable       *bool              `json:"reachable,omitempty"`
	BatteryPct      *int32             `json:"batteryPercentage,omitempty"`
	LinkQuality     *int32             `json:"linkQuality,omitempty"`
	Endpoints       []int32            `json:"endpoints,omitempty"`
}

// +kubebuilder:object:root=true

type MQTTDeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MQTTDevice `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MQTTDevice{}, &MQTTDeviceList{})
}
