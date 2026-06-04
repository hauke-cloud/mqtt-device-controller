package mqtt

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	iov1 "github.com/hauke-cloud/mqtt-device-controller/api/v1alpha1"
	"github.com/hauke-cloud/mqtt-device-controller/internal/device"
	"github.com/hauke-cloud/mqtt-device-controller/internal/metrics"
)

// BridgeClient manages a single MQTT connection to one Tasmota bridge.
type BridgeClient struct {
	bridge  iov1.MQTTBridge
	client  pahomqtt.Client
	log     *slog.Logger
	metrics *metrics.Metrics

	mu           sync.Mutex
	zbStatus1Ch  chan []device.ZbStatus1Item
	zbStatus3Chs map[string]chan device.ZbStatus3Item // keyed by device friendly name

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// newBridgeClient creates and connects a BridgeClient. It does not start the discovery loop.
func newBridgeClient(ctx context.Context, bridge iov1.MQTTBridge, username, password string, log *slog.Logger, m *metrics.Metrics) (*BridgeClient, error) {
	bc := &BridgeClient{
		bridge:       bridge,
		log:          log.With("bridge", bridge.Spec.BridgeName),
		metrics:      m,
		zbStatus1Ch:  make(chan []device.ZbStatus1Item, 1),
		zbStatus3Chs: make(map[string]chan device.ZbStatus3Item),
	}

	opts := pahomqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://%s:%d", bridge.Spec.Host, effectivePort(bridge.Spec.Port))).
		SetClientID(fmt.Sprintf("mqtt-device-controller-%s", bridge.Spec.BridgeName)).
		SetAutoReconnect(true).
		SetMaxReconnectInterval(time.Duration(effectiveBackoff(bridge.Spec.MaxReconnectBackoffSeconds)) * time.Second).
		SetOnConnectHandler(bc.onConnect).
		SetConnectionLostHandler(bc.onConnectionLost)

	if username != "" {
		opts.SetUsername(username).SetPassword(password)
	}

	bc.client = pahomqtt.NewClient(opts)
	token := bc.client.Connect()
	if !token.WaitTimeout(15 * time.Second) {
		return nil, fmt.Errorf("connect to bridge %q: timeout", bridge.Spec.BridgeName)
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("connect to bridge %q: %w", bridge.Spec.BridgeName, err)
	}

	m.BridgeConnected.WithLabelValues(bridge.Spec.BridgeName).Set(1)
	return bc, nil
}

func (bc *BridgeClient) onConnect(_ pahomqtt.Client) {
	bc.log.Info("connected to MQTT broker")
	bc.metrics.BridgeConnected.WithLabelValues(bc.bridge.Spec.BridgeName).Set(1)
	bc.subscribe()
}

func (bc *BridgeClient) onConnectionLost(_ pahomqtt.Client, err error) {
	bc.log.Warn("lost connection to MQTT broker", "err", err)
	bc.metrics.BridgeConnected.WithLabelValues(bc.bridge.Spec.BridgeName).Set(0)
}

func (bc *BridgeClient) subscribe() {
	resultTopic := fmt.Sprintf("stat/%s/RESULT", bc.bridge.Spec.BridgeName)
	token := bc.client.Subscribe(resultTopic, 1, bc.handleMessage)
	if token.Wait() && token.Error() != nil {
		bc.log.Error("subscribe failed", "topic", resultTopic, "err", token.Error())
	}
}

func (bc *BridgeClient) handleMessage(_ pahomqtt.Client, msg pahomqtt.Message) {
	bc.metrics.MessagesReceived.WithLabelValues(bc.bridge.Spec.BridgeName, "result").Inc()

	kind, err := ParseResultPayload(msg.Payload())
	if err != nil {
		bc.log.Debug("could not parse RESULT payload", "err", err)
		return
	}

	switch kind {
	case MessageKindZbStatus1:
		items, err := ParseZbStatus1(msg.Payload())
		if err != nil {
			bc.log.Warn("parse ZbStatus1 failed", "err", err)
			return
		}
		select {
		case bc.zbStatus1Ch <- items:
		default:
			bc.log.Debug("zbStatus1Ch full, dropping response")
		}

	case MessageKindZbStatus3:
		items, err := ParseZbStatus3(msg.Payload())
		if err != nil {
			bc.log.Warn("parse ZbStatus3 failed", "err", err)
			return
		}
		bc.mu.Lock()
		defer bc.mu.Unlock()
		for _, item := range items {
			if ch, ok := bc.zbStatus3Chs[item.Name]; ok {
				select {
				case ch <- item:
				default:
				}
			} else {
				bc.log.Debug("no pending ZbStatus3 waiter for device", "device", item.Name)
			}
		}

	case MessageKindUnknown:
		bc.log.Debug("ignoring unknown RESULT payload")
	}
}

// SendZbStatus1 publishes a ZbStatus1 discovery request and waits for the response.
func (bc *BridgeClient) SendZbStatus1(ctx context.Context, timeout time.Duration) ([]device.ZbStatus1Item, error) {
	topic := fmt.Sprintf("cmnd/%s/ZbStatus1", bc.bridge.Spec.BridgeName)
	token := bc.client.Publish(topic, 1, false, "")
	if !token.WaitTimeout(5 * time.Second) {
		return nil, fmt.Errorf("publish ZbStatus1: timeout")
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("publish ZbStatus1: %w", err)
	}

	select {
	case items := <-bc.zbStatus1Ch:
		return items, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("ZbStatus1 response timeout after %s", timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SendZbStatus3 publishes a ZbStatus3 request for a named device and waits for the response.
func (bc *BridgeClient) SendZbStatus3(ctx context.Context, deviceName string, timeout time.Duration) (device.ZbStatus3Item, error) {
	ch := make(chan device.ZbStatus3Item, 1)
	bc.mu.Lock()
	bc.zbStatus3Chs[deviceName] = ch
	bc.mu.Unlock()
	defer func() {
		bc.mu.Lock()
		delete(bc.zbStatus3Chs, deviceName)
		bc.mu.Unlock()
	}()

	topic := fmt.Sprintf("cmnd/%s/ZbStatus3", bc.bridge.Spec.BridgeName)
	token := bc.client.Publish(topic, 1, false, deviceName)
	if !token.WaitTimeout(5 * time.Second) {
		return device.ZbStatus3Item{}, fmt.Errorf("publish ZbStatus3: timeout")
	}
	if err := token.Error(); err != nil {
		return device.ZbStatus3Item{}, fmt.Errorf("publish ZbStatus3: %w", err)
	}

	select {
	case item := <-ch:
		return item, nil
	case <-time.After(timeout):
		return device.ZbStatus3Item{}, fmt.Errorf("ZbStatus3 response timeout for device %q", deviceName)
	case <-ctx.Done():
		return device.ZbStatus3Item{}, ctx.Err()
	}
}

// SendRename sends a zbname rename command: payload is "<shortAddr>,<newName>".
func (bc *BridgeClient) SendRename(ctx context.Context, shortAddr, newName string) error {
	topic := fmt.Sprintf("cmnd/%s/zbname", bc.bridge.Spec.BridgeName)
	payload := fmt.Sprintf("%s,%s", shortAddr, newName)
	token := bc.client.Publish(topic, 1, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish zbname: timeout")
	}
	return token.Error()
}

// Disconnect cleanly shuts down the client.
func (bc *BridgeClient) Disconnect() {
	bc.client.Disconnect(500)
	bc.metrics.BridgeConnected.WithLabelValues(bc.bridge.Spec.BridgeName).Set(0)
}

func effectivePort(p int32) int32 {
	if p == 0 {
		return 1883
	}
	return p
}

func effectiveBackoff(s int32) int32 {
	if s == 0 {
		return 60
	}
	return s
}
