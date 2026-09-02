package main

import (
	"testing"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
	"github.com/jdanielsobrado/grownerve/internal/edge"
)

const simulatorCommandID = "01990a20-6a00-7000-8000-000000000099"

func TestRejectedCommandReplayStaysRejected(t *testing.T) {
	now := time.Date(2026, 9, 2, 5, 0, 0, 0, time.UTC)
	controller := edge.NewController(defaultDeviceID)
	replay := newSimulatedCommandReplay()
	command := deviceprotocol.Command{
		ProtocolVersion: deviceprotocol.Version,
		CommandID: simulatorCommandID, TargetChannelID: defaultChannels[0],
		Type: "unsupported", Value: 50,
		IssuedAt: now, ExpiresAt: now.Add(time.Minute),
	}
	first := replay.apply(controller, command, now)
	if first.result != "rejected" {
		t.Fatalf("first result = %q, want rejected", first.result)
	}

	command.Type = "set_percent"
	second := replay.apply(controller, command, now)
	if second.result != first.result || second.reason != first.reason {
		t.Fatalf("duplicate changed terminal result: first=%+v second=%+v", first, second)
	}
}

func TestAppliedCommandReplayDoesNotExtendOverride(t *testing.T) {
	now := time.Date(2026, 9, 2, 5, 0, 0, 0, time.UTC)
	controller := edge.NewController(defaultDeviceID)
	replay := newSimulatedCommandReplay()
	command := deviceprotocol.Command{
		ProtocolVersion: deviceprotocol.Version,
		CommandID: simulatorCommandID, TargetChannelID: defaultChannels[0],
		Type: "set_percent", Value: 50,
		IssuedAt: now, ExpiresAt: now.Add(30 * time.Second),
	}
	if result := replay.apply(controller, command, now); result.result != "applied" {
		t.Fatalf("first result = %+v", result)
	}

	command.Value = 90
	command.ExpiresAt = now.Add(5 * time.Minute)
	if result := replay.apply(controller, command, now.Add(20*time.Second)); result.result != "applied" {
		t.Fatalf("duplicate result = %+v", result)
	}
	resolved := controller.Resolve(defaultChannels[0], now.Add(31*time.Second))
	if resolved.Value != 0 {
		t.Fatalf("duplicate command extended the original override: %+v", resolved)
	}
}
