package mqtt

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hauke-cloud/mqtt-device-controller/internal/device"
)

// TopicParts holds the parsed components of a Tasmota MQTT topic.
type TopicParts struct {
	Prefix     string // tele, stat, cmnd
	BridgeName string
	Suffix     string // SENSOR, RESULT, etc.
}

// ParseTopic splits a topic like "tele/my-bridge/SENSOR" into its components.
func ParseTopic(topic string) (TopicParts, error) {
	parts := strings.SplitN(topic, "/", 3)
	if len(parts) != 3 {
		return TopicParts{}, fmt.Errorf("unexpected topic format: %q", topic)
	}
	return TopicParts{
		Prefix:     parts[0],
		BridgeName: parts[1],
		Suffix:     parts[2],
	}, nil
}

// MessageKind identifies what type of Tasmota message a payload contains.
type MessageKind int

const (
	MessageKindUnknown   MessageKind = iota
	MessageKindZbStatus1             // device list
	MessageKindZbStatus3             // device details
	MessageKindRename                // rename ACK
)

// ParseResultPayload inspects the top-level key of a stat/+/RESULT payload
// and returns the kind along with the raw bytes for further decoding.
func ParseResultPayload(data []byte) (MessageKind, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return MessageKindUnknown, fmt.Errorf("unmarshal result payload: %w", err)
	}
	if _, ok := probe["ZbStatus1"]; ok {
		return MessageKindZbStatus1, nil
	}
	if _, ok := probe["ZbStatus3"]; ok {
		return MessageKindZbStatus3, nil
	}
	// Rename ACK: top-level keys are short addresses like "0xED0B"
	for k := range probe {
		if strings.HasPrefix(k, "0x") {
			return MessageKindRename, nil
		}
	}
	return MessageKindUnknown, nil
}

// ParseZbStatus1 decodes a ZbStatus1 device-list payload.
func ParseZbStatus1(data []byte) ([]device.ZbStatus1Item, error) {
	var resp device.ZbStatus1Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse ZbStatus1: %w", err)
	}
	return resp.ZbStatus1, nil
}

// ParseZbStatus3 decodes a ZbStatus3 device-detail payload.
func ParseZbStatus3(data []byte) ([]device.ZbStatus3Item, error) {
	var resp device.ZbStatus3Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse ZbStatus3: %w", err)
	}
	return resp.ZbStatus3, nil
}
