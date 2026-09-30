package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

// The z2m mode impersonates a Zigbee2MQTT 2.x bridge with three devices so the
// Zigbee integration path (discovery, adoption, telemetry, commands and their
// reflected state) can be exercised without a coordinator or radios.
const (
	simulatedSensorIEEE = "0x00158d0000c0ffee"
	simulatedPlugIEEE   = "0x00158d0000beef01"
	simulatedLightIEEE  = "0x00158d0000beef02"
)

const simulatedBridgeDevices = `[
 {"ieee_address":"0x00124b0000000000","type":"Coordinator","friendly_name":"Coordinator"},
 {"ieee_address":"` + simulatedSensorIEEE + `","type":"EndDevice","friendly_name":"Sim climate","power_source":"Battery","software_build_id":"sim",
  "definition":{"model":"SIM-TH","vendor":"GrowNerve simulator","description":"Simulated temperature and humidity sensor","exposes":[
   {"type":"numeric","name":"temperature","property":"temperature","access":1,"unit":"°C"},
   {"type":"numeric","name":"humidity","property":"humidity","access":1,"unit":"%"},
   {"type":"numeric","name":"battery","property":"battery","access":1,"unit":"%","category":"diagnostic"},
   {"type":"numeric","name":"linkquality","property":"linkquality","access":1,"unit":"lqi","category":"diagnostic"}]}},
 {"ieee_address":"` + simulatedPlugIEEE + `","type":"Router","friendly_name":"Sim plug","power_source":"Mains (single phase)","software_build_id":"sim",
  "definition":{"model":"SIM-PLUG","vendor":"GrowNerve simulator","description":"Simulated smart plug","exposes":[
   {"type":"switch","features":[{"type":"binary","name":"state","property":"state","access":7,"value_on":"ON","value_off":"OFF"}]},
   {"type":"numeric","name":"power","property":"power","access":1,"unit":"W"}]}},
 {"ieee_address":"` + simulatedLightIEEE + `","type":"Router","friendly_name":"Sim light","power_source":"Mains (single phase)","software_build_id":"sim",
  "definition":{"model":"SIM-LIGHT","vendor":"GrowNerve simulator","description":"Simulated dimmable grow light","exposes":[
   {"type":"light","features":[
    {"type":"binary","name":"state","property":"state","access":7,"value_on":"ON","value_off":"OFF"},
    {"type":"numeric","name":"brightness","property":"brightness","access":7,"value_min":0,"value_max":254}]}]}}
]`

type simulatedZigbee struct {
	mu         sync.Mutex
	client     paho.Client
	base       string
	plugOn     bool
	lightOn    bool
	brightness float64
	tick       float64
}

func runZigbee2MQTT(ctx context.Context, broker, username, password, base string, interval time.Duration) error {
	bridge := &simulatedZigbee{base: base, lightOn: true, brightness: 200}
	options := paho.NewClientOptions().AddBroker(broker).SetClientID("grownerve-simulator-z2m").SetAutoReconnect(true).
		SetWill(base+"/bridge/state", `{"state":"offline"}`, 1, true)
	if username != "" {
		options.SetUsername(username).SetPassword(password)
	}
	options.SetOnConnectHandler(func(client paho.Client) {
		client.Subscribe(base+"/+/set", 1, bridge.handleSet)
		client.Subscribe(base+"/bridge/request/permit_join", 1, bridge.handlePermitJoin)
		bridge.announce()
	})
	bridge.client = paho.NewClient(options)
	if token := bridge.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	defer func() {
		bridge.client.Publish(base+"/bridge/state", 1, true, `{"state":"offline"}`).Wait()
		bridge.client.Disconnect(250)
	}()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	bridge.publishStates()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			bridge.publishStates()
		}
	}
}

func (bridge *simulatedZigbee) publish(topic string, retained bool, payload any) {
	encoded, _ := json.Marshal(payload)
	bridge.client.Publish(bridge.base+"/"+topic, 1, retained, encoded)
}

func (bridge *simulatedZigbee) announce() {
	bridge.client.Publish(bridge.base+"/bridge/state", 1, true, `{"state":"online"}`)
	bridge.client.Publish(bridge.base+"/bridge/devices", 1, true, simulatedBridgeDevices)
	for _, name := range []string{"Sim climate", "Sim plug", "Sim light"} {
		bridge.client.Publish(bridge.base+"/"+name+"/availability", 1, true, `{"state":"online"}`)
	}
}

func (bridge *simulatedZigbee) publishStates() {
	bridge.mu.Lock()
	bridge.tick++
	wave := math.Sin(bridge.tick / 6)
	climate := map[string]any{"temperature": math.Round((23.1+wave)*10) / 10, "humidity": math.Round(62 + wave*4), "battery": 91, "linkquality": 132}
	plug, light := bridge.plugStateLocked(), bridge.lightStateLocked()
	bridge.mu.Unlock()
	bridge.publish("Sim climate", false, climate)
	bridge.publish("Sim plug", false, plug)
	bridge.publish("Sim light", false, light)
}

func onOff(on bool) string {
	if on {
		return "ON"
	}
	return "OFF"
}

func (bridge *simulatedZigbee) plugStateLocked() map[string]any {
	power := 0.0
	if bridge.plugOn {
		power = 38.5
	}
	return map[string]any{"state": onOff(bridge.plugOn), "power": power}
}

func (bridge *simulatedZigbee) lightStateLocked() map[string]any {
	return map[string]any{"state": onOff(bridge.lightOn), "brightness": bridge.brightness}
}

// handleSet applies a request and reports the resulting state, as a real
// device does through Zigbee2MQTT.
func (bridge *simulatedZigbee) handleSet(_ paho.Client, message paho.Message) {
	name := strings.TrimSuffix(strings.TrimPrefix(message.Topic(), bridge.base+"/"), "/set")
	var request map[string]any
	if json.Unmarshal(message.Payload(), &request) != nil {
		return
	}
	bridge.mu.Lock()
	var state map[string]any
	switch name {
	case "Sim plug":
		if value, ok := request["state"].(string); ok {
			bridge.plugOn = strings.EqualFold(value, "ON")
		}
		state = bridge.plugStateLocked()
	case "Sim light":
		if value, ok := request["state"].(string); ok {
			bridge.lightOn = strings.EqualFold(value, "ON")
		}
		if value, ok := request["brightness"].(float64); ok {
			bridge.brightness = math.Max(0, math.Min(254, value))
			bridge.lightOn = bridge.brightness > 0
		}
		state = bridge.lightStateLocked()
	}
	bridge.mu.Unlock()
	if state != nil {
		time.AfterFunc(150*time.Millisecond, func() { bridge.publish(name, false, state) })
	}
}

func (bridge *simulatedZigbee) handlePermitJoin(_ paho.Client, message paho.Message) {
	var request struct {
		Time int `json:"time"`
	}
	if json.Unmarshal(message.Payload(), &request) != nil {
		return
	}
	info := map[string]any{"permit_join": request.Time > 0, "permit_join_end": nil}
	if request.Time > 0 {
		info["permit_join_end"] = time.Now().Add(time.Duration(request.Time) * time.Second).UnixMilli()
	}
	bridge.publish("bridge/response/permit_join", false, map[string]any{"status": "ok", "data": map[string]int{"time": request.Time}})
	bridge.publish("bridge/info", true, info)
	fmt.Printf("permit join for %ds\n", request.Time)
}
