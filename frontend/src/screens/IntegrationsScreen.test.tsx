import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { pilotData } from "../runtime/pilotData";
import type { DiscoveredDevice, IntegrationClient, IntegrationStatus } from "../runtime/integrations";
import { plug } from "../test/integrationFixtures";
import { IntegrationsScreen } from "./IntegrationsScreen";

// The screen loads statuses, then devices; allow for a busy parallel test run.
const slow = { timeout: 5000 };

const statuses: IntegrationStatus[] = [
  { provider: "zigbee2mqtt", enabled: true, state: "connected", since: "2026-09-30T06:00:00Z", device_count: 2, adopted_count: 1, permit_join_supported: true },
  { provider: "matter", enabled: false, state: "disabled", since: "0001-01-01T00:00:00Z", device_count: 0, adopted_count: 0, permit_join_supported: false },
  { provider: "home_assistant", enabled: false, state: "disabled", since: "0001-01-01T00:00:00Z", device_count: 0, adopted_count: 0, permit_join_supported: false },
];

const adoptedSensor: DiscoveredDevice = {
  provider: "zigbee2mqtt", external_id: "0x00158d0000c0ffee", name: "Sim climate", suggested_type: "sensor", available: true, adopted_device_id: "device-1",
  capabilities: [{ key: "temperature", label: "Temperature", kind: "measurement", value_type: "number", unit: "degC", suggested_channel_key: "sim_climate.temperature" }],
};

function fakeClient(): IntegrationClient & { [K in keyof IntegrationClient]: ReturnType<typeof vi.fn> } {
  return {
    listIntegrations: vi.fn().mockResolvedValue(statuses),
    listDiscovered: vi.fn().mockResolvedValue([plug, adoptedSensor]),
    adoptDevice: vi.fn().mockResolvedValue(undefined),
    permitJoin: vi.fn().mockResolvedValue("2026-09-30T12:02:00Z"),
  };
}

describe("Integrations screen", () => {
  it("explains that integrations need the server runtime", () => {
    render(<IntegrationsScreen data={pilotData()} runtimeMode="browser" onOpenDevices={() => undefined} />);
    expect(screen.getByText("Integrations need the server runtime.")).toBeVisible();
    expect(screen.getAllByText("Server only")).toHaveLength(3);
  });

  it("shows provider state and the discovered devices of the first enabled provider", async () => {
    const client = fakeClient();
    render(<IntegrationsScreen data={pilotData()} runtimeMode="server" client={client} onOpenDevices={() => undefined} />);
    expect(await screen.findByRole("heading", { name: "Zigbee devices" }, slow)).toBeVisible();
    expect(client.listDiscovered).toHaveBeenCalledWith("zigbee2mqtt");
    expect(screen.getByText("connected")).toBeVisible();
    expect(screen.getAllByText(/Enable/).length).toBe(2);
    expect(await screen.findByRole("button", { name: "Adopt Sim plug" }, slow)).toBeVisible();
    expect(screen.getByRole("button", { name: "Adopted · view" })).toBeVisible();
  });

  it("will not adopt an output until the missing failsafe is acknowledged, then sends the chosen channels", async () => {
    const client = fakeClient(), data = pilotData();
    render(<IntegrationsScreen data={data} runtimeMode="server" client={client} onOpenDevices={() => undefined} />);
    fireEvent.click(await screen.findByRole("button", { name: "Adopt Sim plug" }, slow));

    const submit = screen.getByRole("button", { name: "Adopt device" });
    expect(submit).toBeDisabled();
    expect(screen.getByText("Confirm that controllable outputs have no controller-side failsafe.")).toBeVisible();
    fireEvent.click(screen.getByRole("checkbox", { name: /no controller-side failsafe/ }));
    fireEvent.change(screen.getByLabelText("Power channel key"), { target: { value: "tent.exhaust.watts" } });
    expect(submit).toBeEnabled();
    fireEvent.click(submit);

    await waitFor(() => expect(client.adoptDevice).toHaveBeenCalledOnce(), slow);
    const [provider, externalId, request] = client.adoptDevice.mock.calls[0];
    expect([provider, externalId]).toEqual(["zigbee2mqtt", plug.external_id]);
    expect(request).toMatchObject({
      zone_id: data.zones[0].id, acknowledge_no_edge_failsafe: true,
      channels: [{ capability_key: "state" }, { capability_key: "power", key: "tent.exhaust.watts", entity_type: "zone" }],
    });
    expect(await screen.findByRole("status", undefined, slow)).toHaveTextContent("Sim plug was adopted");
  });

  it("shows the server's refusal instead of closing the form", async () => {
    const client = fakeClient();
    client.adoptDevice.mockRejectedValueOnce(new Error("channel key \"sim_plug.power\" is already in use"));
    render(<IntegrationsScreen data={pilotData()} runtimeMode="server" client={client} onOpenDevices={() => undefined} />);
    fireEvent.click(await screen.findByRole("button", { name: "Adopt Sim plug" }, slow));
    fireEvent.click(screen.getByRole("checkbox", { name: "Use State" }));
    fireEvent.click(screen.getByRole("button", { name: "Adopt device" }));
    expect(await screen.findByRole("alert", undefined, slow)).toHaveTextContent("already in use");
    expect(screen.getByRole("button", { name: "Adopt device" })).toBeVisible();
  });

  it("opens the Zigbee network for pairing", async () => {
    const client = fakeClient();
    render(<IntegrationsScreen data={pilotData()} runtimeMode="server" client={client} onOpenDevices={() => undefined} />);
    const card = (await screen.findByRole("heading", { name: "Zigbee" }, slow)).closest("section")!;
    fireEvent.click(within(card).getByRole("button", { name: /Pair new devices/ }));
    await waitFor(() => expect(client.permitJoin).toHaveBeenCalledWith("zigbee2mqtt", 120), slow);
  });
});
