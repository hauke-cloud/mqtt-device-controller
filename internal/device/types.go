package device

// ZbStatus1Item is one entry from a ZbStatus1 (device list) response.
type ZbStatus1Item struct {
	Device string `json:"Device"`
	Name   string `json:"Name"`
}

// ZbStatus1Response is the full parsed payload of a ZbStatus1 RESULT message.
type ZbStatus1Response struct {
	ZbStatus1 []ZbStatus1Item `json:"ZbStatus1"`
}

// ZbStatus3Item is a single device entry from a ZbStatus3 (device detail) response.
type ZbStatus3Item struct {
	Device              string  `json:"Device"`
	Name                string  `json:"Name"`
	IEEEAddr            string  `json:"IEEEAddr"`
	ModelID             string  `json:"ModelId"`
	Manufacturer        string  `json:"Manufacturer"`
	Endpoints           []int   `json:"Endpoints"`
	Reachable           bool    `json:"Reachable"`
	BatteryPercentage   int     `json:"BatteryPercentage"`
	LinkQuality         int     `json:"LinkQuality"`
	LastSeen            int64   `json:"LastSeen"`
	LastSeenEpoch       int64   `json:"LastSeenEpoch"`
	BatteryLastSeenEpoch int64  `json:"BatteryLastSeenEpoch"`
	Temperature         float64 `json:"Temperature"`
	Pressure            float64 `json:"Pressure"`
	Humidity            float64 `json:"Humidity"`
}

// ZbStatus3Response is the full parsed payload of a ZbStatus3 RESULT message.
type ZbStatus3Response struct {
	ZbStatus3 []ZbStatus3Item `json:"ZbStatus3"`
}

// ZbReceivedPayload is a spontaneous live report on tele/+/SENSOR.
type ZbReceivedPayload struct {
	ZbReceived map[string]ZbReceivedItem `json:"ZbReceived"`
}

// ZbReceivedItem is one device's live report inside ZbReceived.
type ZbReceivedItem struct {
	Device            string  `json:"Device"`
	Name              string  `json:"Name"`
	Endpoint          int     `json:"Endpoint"`
	LinkQuality       int     `json:"LinkQuality"`
	BatteryPercentage int     `json:"BatteryPercentage"`
	StateValve        *int    `json:"StateValve,omitempty"`
	Temperature       float64 `json:"Temperature,omitempty"`
	Humidity          float64 `json:"Humidity,omitempty"`
	Pressure          float64 `json:"Pressure,omitempty"`
}

// RenameResponse is returned on stat/<bridge>/RESULT after a zbname command.
type RenameResponse map[string]struct {
	Name string `json:"Name"`
}
