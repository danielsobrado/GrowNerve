import type { DiscoveredDevice } from "../runtime/integrations";

/** Devices as the simulated Zigbee2MQTT bridge (device-simulator -mode z2m) reports them. */
export const plug: DiscoveredDevice = {
  provider: "zigbee2mqtt", external_id: "0x00158d0000beef01", name: "Sim plug", suggested_type: "controller", available: true, power_source: "mains",
  capabilities: [
    { key: "state", label: "State", kind: "command", value_type: "boolean", unit: "bool", minimum: 0, maximum: 1, suggested_channel_key: "sim_plug.state" },
    { key: "power", label: "Power", kind: "measurement", value_type: "number", unit: "W", suggested_channel_key: "sim_plug.power" },
    { key: "linkquality", label: "Linkquality", kind: "measurement", value_type: "number", unit: "lqi", suggested_channel_key: "sim_plug.linkquality", diagnostic: true },
  ],
};

export const light: DiscoveredDevice = {
  provider: "zigbee2mqtt", external_id: "0x00158d0000beef02", name: "Sim light", suggested_type: "light", available: true,
  capabilities: [{ key: "brightness", label: "Brightness", kind: "command", value_type: "number", unit: "%", minimum: 0, maximum: 100, suggested_channel_key: "sim_light.brightness" }],
};
