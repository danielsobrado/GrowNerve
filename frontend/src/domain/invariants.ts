import type { Alert, Channel, FarmCommand, GrowCycle, Measurement, RecipeVersion, Setpoint } from "./model";

export class DomainError extends Error {
  constructor(public readonly code: string, message: string) {
    super(message);
    this.name = "DomainError";
  }
}

export function startGrowCycle(grow: GrowCycle, recipe: RecipeVersion | undefined, actualStart: string): GrowCycle {
  if (grow.status !== "planned") throw new DomainError("GROW_CYCLE_NOT_PLANNED", "Only a planned grow can be started");
  if (!recipe || recipe.id !== grow.recipe_version_id || recipe.status !== "published") throw new DomainError("RECIPE_VERSION_NOT_PUBLISHED", "Grow cycle requires its assigned published recipe version");
  return { ...grow, status: "active", actual_start: actualStart };
}

export function completeGrowCycle(grow: GrowCycle, completedAt: string): GrowCycle {
  if (grow.status !== "active" || !grow.actual_start) throw new DomainError("GROW_CYCLE_NOT_ACTIVE", "Only an active grow can be completed");
  if (new Date(completedAt) < new Date(grow.actual_start)) throw new DomainError("HARVEST_BEFORE_GROW_START", "Harvest cannot precede grow start");
  return { ...grow, status: "completed", completed_at: completedAt };
}

export function evaluateSetpoint(value: number, setpoint: Pick<Setpoint, "minimum" | "maximum">): "low" | "in_range" | "high" | "unknown" {
  if (setpoint.minimum !== undefined && value < setpoint.minimum) return "low";
  if (setpoint.maximum !== undefined && value > setpoint.maximum) return "high";
  if (setpoint.minimum === undefined && setpoint.maximum === undefined) return "unknown";
  return "in_range";
}

export function validateMeasurement(measurement: Measurement, channel: Channel): Measurement {
  if (channel.kind !== "measurement") throw new DomainError("CHANNEL_NOT_MEASUREMENT", "Target channel does not accept measurements");
  if (channel.unit !== measurement.unit) throw new DomainError("INVALID_UNIT_DIMENSION", `Measurement unit ${measurement.unit} does not match channel unit ${channel.unit}`);
  if (!Number.isFinite(measurement.value)) throw new DomainError("INVALID_MEASUREMENT", "Measurement value must be finite");
  if (channel.minimum !== undefined && measurement.value < channel.minimum || channel.maximum !== undefined && measurement.value > channel.maximum) {
    return { ...measurement, quality: "suspect" };
  }
  return measurement;
}

export function acknowledgeAlert(alert: Alert, actor: string, at: string): Alert {
  if (alert.status !== "open") throw new DomainError("ALERT_NOT_OPEN", "Only an open alert can be acknowledged");
  return { ...alert, status: "acknowledged", acknowledged_by: actor, acknowledged_at: at };
}

export function resolveAlert(alert: Alert, at: string): Alert {
  if (alert.status === "resolved") throw new DomainError("ALERT_ALREADY_RESOLVED", "Alert is already resolved");
  return { ...alert, status: "resolved", resolved_at: at };
}

const transitions: Record<FarmCommand["status"], FarmCommand["status"][]> = {
  pending: ["published", "rejected", "cancelled"],
  published: ["acknowledged", "rejected", "timed_out"],
  acknowledged: ["applied", "rejected", "timed_out"],
  applied: [], rejected: [], timed_out: [], cancelled: [],
};

export function transitionCommand(command: FarmCommand, next: FarmCommand["status"], at: string): FarmCommand {
  if (!transitions[command.status].includes(next)) throw new DomainError("INVALID_COMMAND_TRANSITION", `Invalid command transition ${command.status} -> ${next}`);
  return { ...command, status: next, updated_at: at };
}
