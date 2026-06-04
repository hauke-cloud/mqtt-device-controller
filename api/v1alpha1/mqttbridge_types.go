package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=mb
// +kubebuilder:printcolumn:name="Bridge",type="string",JSONPath=".spec.bridgeName"
// +kubebuilder:printcolumn:name="Host",type="string",JSONPath=".spec.host"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.connectionState"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// MQTTBridge is read-only for this controller; it is managed by a separate operator.
type MQTTBridge struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MQTTBridgeSpec   `json:"spec"`
	Status MQTTBridgeStatus `json:"status,omitempty"`
}

type MQTTBridgeSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	BridgeName string `json:"bridgeName"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`

	// +kubebuilder:default=1883
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`

	// +kubebuilder:default=tasmota
	// +kubebuilder:validation:Enum=tasmota;generic
	DeviceType BridgeDeviceType `json:"deviceType,omitempty"`

	// +kubebuilder:default=false
	DiscoveryEnabled bool `json:"discoveryEnabled,omitempty"`

	// +kubebuilder:default=60
	// +kubebuilder:validation:Minimum=5
	// +kubebuilder:validation:Maximum=300
	MaxReconnectBackoffSeconds int32 `json:"maxReconnectBackoffSeconds,omitempty"`

	CredentialsSecretRef *CredentialsSecretRef `json:"credentialsSecretRef,omitempty"`
	Topics               []TopicConfig         `json:"topics,omitempty"`
}

type BridgeDeviceType string

const (
	BridgeDeviceTypeTasmota BridgeDeviceType = "tasmota"
	BridgeDeviceTypeGeneric BridgeDeviceType = "generic"
)

type CredentialsSecretRef struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`

	// Key in the Secret whose value is used as the MQTT username. Defaults to "username".
	UsernameKey string `json:"usernameKey,omitempty"`

	// Key in the Secret whose value is used as the MQTT password. Defaults to "password".
	PasswordKey string `json:"passwordKey,omitempty"`
}

type TopicConfig struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Topic string `json:"topic"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=telemetry;result;state;command
	Type TopicType `json:"type"`

	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=2
	QoS int32 `json:"qos,omitempty"`
}

type TopicType string

const (
	TopicTypeTelemetry TopicType = "telemetry"
	TopicTypeResult    TopicType = "result"
	TopicTypeState     TopicType = "state"
	TopicTypeCommand   TopicType = "command"
)

type ConnectionState string

const (
	ConnectionStateConnected    ConnectionState = "connected"
	ConnectionStateDisconnected ConnectionState = "disconnected"
	ConnectionStateConnecting   ConnectionState = "connecting"
	ConnectionStateError        ConnectionState = "error"
)

type MQTTBridgeStatus struct {
	Conditions           []metav1.Condition `json:"conditions,omitempty"`
	ConnectionState      ConnectionState    `json:"connectionState,omitempty"`
	ErrorMessage         string             `json:"errorMessage,omitempty"`
	LastConnectedTime    *metav1.Time       `json:"lastConnectedTime,omitempty"`
	LastDisconnectedTime *metav1.Time       `json:"lastDisconnectedTime,omitempty"`
	MessagesReceived     int64              `json:"messagesReceived,omitempty"`
	ReconnectCount       int32              `json:"reconnectCount,omitempty"`
}

// +kubebuilder:object:root=true

type MQTTBridgeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MQTTBridge `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MQTTBridge{}, &MQTTBridgeList{})
}
