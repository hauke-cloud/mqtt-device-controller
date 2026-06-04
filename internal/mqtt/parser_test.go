package mqtt

import (
	"testing"
)

func TestParseTopic(t *testing.T) {
	tests := []struct {
		name       string
		topic      string
		wantPrefix string
		wantBridge string
		wantSuffix string
		wantErr    bool
	}{
		{
			name:       "tele sensor",
			topic:      "tele/my-bridge/SENSOR",
			wantPrefix: "tele",
			wantBridge: "my-bridge",
			wantSuffix: "SENSOR",
		},
		{
			name:       "stat result",
			topic:      "stat/tasmota_office/RESULT",
			wantPrefix: "stat",
			wantBridge: "tasmota_office",
			wantSuffix: "RESULT",
		},
		{
			name:    "too few segments",
			topic:   "tele/SENSOR",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTopic(tt.topic)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseTopic(%q) error = %v, wantErr %v", tt.topic, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.Prefix != tt.wantPrefix {
				t.Errorf("Prefix = %q, want %q", got.Prefix, tt.wantPrefix)
			}
			if got.BridgeName != tt.wantBridge {
				t.Errorf("BridgeName = %q, want %q", got.BridgeName, tt.wantBridge)
			}
			if got.Suffix != tt.wantSuffix {
				t.Errorf("Suffix = %q, want %q", got.Suffix, tt.wantSuffix)
			}
		})
	}
}

func TestParseResultPayload_ZbStatus1(t *testing.T) {
	payload := []byte(`{"ZbStatus1":[{"Device":"0x4F2E","Name":"valve-right"},{"Device":"0xDA76","Name":"valve-center"}]}`)
	kind, err := ParseResultPayload(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != MessageKindZbStatus1 {
		t.Errorf("kind = %v, want ZbStatus1", kind)
	}
}

func TestParseResultPayload_ZbStatus3(t *testing.T) {
	payload := []byte(`{"ZbStatus3":[{"Device":"0x3A65","Name":"room-floor3","IEEEAddr":"0x00158D008C7CCC7B","ModelId":"lumi.weather","Reachable":true,"BatteryPercentage":100,"LinkQuality":69}]}`)
	kind, err := ParseResultPayload(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != MessageKindZbStatus3 {
		t.Errorf("kind = %v, want ZbStatus3", kind)
	}
}

func TestParseResultPayload_Rename(t *testing.T) {
	payload := []byte(`{"0xED0B":{"Name":"room-office-romy"}}`)
	kind, err := ParseResultPayload(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != MessageKindRename {
		t.Errorf("kind = %v, want Rename", kind)
	}
}

func TestParseZbStatus1(t *testing.T) {
	payload := []byte(`{"ZbStatus1":[{"Device":"0x4F2E","Name":"valve-right"},{"Device":"0xDA76","Name":"valve-center"}]}`)
	items, err := ParseZbStatus1(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Name != "valve-right" {
		t.Errorf("items[0].Name = %q, want valve-right", items[0].Name)
	}
	if items[1].Device != "0xDA76" {
		t.Errorf("items[1].Device = %q, want 0xDA76", items[1].Device)
	}
}

func TestParseZbStatus3(t *testing.T) {
	payload := []byte(`{"ZbStatus3":[{"Device":"0x3A65","Name":"room-floor3","IEEEAddr":"0x00158D008C7CCC7B","ModelId":"lumi.weather","Manufacturer":"LUMI","Endpoints":[1],"Temperature":26.31,"Pressure":1019,"Humidity":45.67,"Reachable":true,"BatteryPercentage":100,"LinkQuality":69}]}`)
	items, err := ParseZbStatus3(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	d := items[0]
	if d.Name != "room-floor3" {
		t.Errorf("Name = %q, want room-floor3", d.Name)
	}
	if d.ModelID != "lumi.weather" {
		t.Errorf("ModelID = %q, want lumi.weather", d.ModelID)
	}
	if !d.Reachable {
		t.Error("Reachable = false, want true")
	}
	if d.BatteryPercentage != 100 {
		t.Errorf("BatteryPercentage = %d, want 100", d.BatteryPercentage)
	}
	if d.Temperature != 26.31 {
		t.Errorf("Temperature = %f, want 26.31", d.Temperature)
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"valve-right", "valve-right"},
		{"room-office-hauke", "room-office-hauke"},
		{"Sensor_1", "sensor-1"},
		{"  leading-trailing  ", "leading-trailing"},
		{"has spaces", "has-spaces"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
