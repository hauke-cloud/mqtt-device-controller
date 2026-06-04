package v1alpha1

// This file contains hand-written DeepCopyInto implementations for non-root types
// that controller-gen calls but does not generate bodies for (controller-gen v0.17.x
// generates the call sites in root types but skips method bodies for Spec/Status types).

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (in *MQTTBridgeSpec) DeepCopyInto(out *MQTTBridgeSpec) {
	*out = *in
	if in.CredentialsSecretRef != nil {
		in, out := &in.CredentialsSecretRef, &out.CredentialsSecretRef
		*out = new(CredentialsSecretRef)
		**out = **in
	}
	if in.Topics != nil {
		in, out := &in.Topics, &out.Topics
		*out = make([]TopicConfig, len(*in))
		copy(*out, *in)
	}
}

func (in *MQTTBridgeStatus) DeepCopyInto(out *MQTTBridgeStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.LastConnectedTime != nil {
		in, out := &in.LastConnectedTime, &out.LastConnectedTime
		*out = (*in).DeepCopy()
	}
	if in.LastDisconnectedTime != nil {
		in, out := &in.LastDisconnectedTime, &out.LastDisconnectedTime
		*out = (*in).DeepCopy()
	}
}

func (in *MQTTDeviceSpec) DeepCopyInto(out *MQTTDeviceSpec) {
	*out = *in
	out.BridgeRef = in.BridgeRef
	if in.TypeRef != nil {
		in, out := &in.TypeRef, &out.TypeRef
		*out = new(TypeRef)
		**out = **in
	}
}

func (in *MQTTDeviceStatus) DeepCopyInto(out *MQTTDeviceStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.LastSeenTime != nil {
		in, out := &in.LastSeenTime, &out.LastSeenTime
		*out = (*in).DeepCopy()
	}
	if in.LastUpdatedTime != nil {
		in, out := &in.LastUpdatedTime, &out.LastUpdatedTime
		*out = (*in).DeepCopy()
	}
	if in.Reachable != nil {
		in, out := &in.Reachable, &out.Reachable
		*out = new(bool)
		**out = **in
	}
	if in.BatteryPct != nil {
		in, out := &in.BatteryPct, &out.BatteryPct
		*out = new(int32)
		**out = **in
	}
	if in.LinkQuality != nil {
		in, out := &in.LinkQuality, &out.LinkQuality
		*out = new(int32)
		**out = **in
	}
	if in.Endpoints != nil {
		in, out := &in.Endpoints, &out.Endpoints
		*out = make([]int32, len(*in))
		copy(*out, *in)
	}
}
