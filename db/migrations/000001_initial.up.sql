CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE OR REPLACE FUNCTION uuidv7()
RETURNS uuid LANGUAGE sql VOLATILE PARALLEL SAFE AS $$
  WITH parts AS (
    SELECT lpad(to_hex(floor(extract(epoch FROM clock_timestamp()) * 1000)::bigint), 12, '0') AS ts,
           encode(gen_random_bytes(10), 'hex') AS random_hex
  )
  SELECT (substr(ts, 1, 8) || '-' || substr(ts, 9, 4) || '-' || '7' || substr(random_hex, 1, 3) || '-' ||
          substr('89ab', 1 + (get_byte(decode(substr(random_hex, 4, 2), 'hex'), 0) % 4), 1) ||
          substr(random_hex, 6, 3) || '-' || substr(random_hex, 9, 12))::uuid FROM parts
$$;

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN NEW.updated_at = clock_timestamp(); RETURN NEW; END
$$;

CREATE TABLE schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  external_subject TEXT UNIQUE,
  email TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('viewer', 'operator', 'manager', 'administrator')),
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'archived')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE facilities (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  name TEXT NOT NULL,
  timezone TEXT NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE zones (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  facility_id UUID NOT NULL REFERENCES facilities(id),
  parent_zone_id UUID REFERENCES zones(id),
  name TEXT NOT NULL,
  type TEXT NOT NULL CHECK (type IN ('room', 'tent', 'rack', 'level')),
  version BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (facility_id, parent_zone_id, name)
);

CREATE TABLE crops (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE varieties (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  crop_id UUID NOT NULL REFERENCES crops(id),
  code TEXT NOT NULL,
  name TEXT NOT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  UNIQUE (crop_id, code)
);

CREATE TABLE reservoirs (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  zone_id UUID NOT NULL REFERENCES zones(id),
  name TEXT NOT NULL,
  nominal_capacity_l NUMERIC(12,3) NOT NULL CHECK (nominal_capacity_l > 0),
  working_volume_l NUMERIC(12,3) CHECK (working_volume_l >= 0 AND working_volume_l <= nominal_capacity_l),
  version BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE grow_recipes (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  crop_id UUID NOT NULL REFERENCES crops(id),
  variety_id UUID REFERENCES varieties(id),
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE grow_recipe_versions (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  recipe_id UUID NOT NULL REFERENCES grow_recipes(id),
  version INTEGER NOT NULL CHECK (version > 0),
  status TEXT NOT NULL CHECK (status IN ('draft', 'published', 'retired')),
  published_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (recipe_id, version),
  CHECK ((status = 'published' AND published_at IS NOT NULL) OR status <> 'published')
);

CREATE TABLE recipe_stages (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  recipe_version_id UUID NOT NULL REFERENCES grow_recipe_versions(id),
  key TEXT NOT NULL,
  name TEXT NOT NULL,
  sort_order INTEGER NOT NULL,
  guidance_days INTEGER CHECK (guidance_days > 0),
  UNIQUE (recipe_version_id, key),
  UNIQUE (recipe_version_id, sort_order)
);

CREATE TABLE stage_setpoints (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  stage_id UUID NOT NULL REFERENCES recipe_stages(id),
  channel_key TEXT NOT NULL,
  unit TEXT NOT NULL,
  minimum NUMERIC,
  maximum NUMERIC,
  warning_duration INTERVAL,
  stale_after INTERVAL NOT NULL,
  UNIQUE (stage_id, channel_key),
  CHECK (minimum IS NOT NULL OR maximum IS NOT NULL),
  CHECK (minimum IS NULL OR maximum IS NULL OR minimum <= maximum)
);

CREATE TABLE stage_schedules (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  stage_id UUID NOT NULL REFERENCES recipe_stages(id),
  kind TEXT NOT NULL,
  definition JSONB NOT NULL
);

CREATE TABLE grow_cycles (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  facility_id UUID NOT NULL REFERENCES facilities(id),
  crop_id UUID NOT NULL REFERENCES crops(id),
  variety_id UUID NOT NULL REFERENCES varieties(id),
  recipe_version_id UUID REFERENCES grow_recipe_versions(id),
  name TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('planned', 'active', 'completed', 'abandoned')),
  current_stage_key TEXT,
  planned_start DATE,
  actual_start TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  plant_count INTEGER NOT NULL CHECK (plant_count >= 0),
  notes TEXT NOT NULL DEFAULT '',
  version BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (completed_at IS NULL OR actual_start IS NULL OR completed_at >= actual_start)
);

CREATE TABLE grow_cycle_zones (grow_cycle_id UUID NOT NULL REFERENCES grow_cycles(id), zone_id UUID NOT NULL REFERENCES zones(id), PRIMARY KEY (grow_cycle_id, zone_id));
CREATE TABLE grow_cycle_reservoirs (grow_cycle_id UUID NOT NULL REFERENCES grow_cycles(id), reservoir_id UUID NOT NULL REFERENCES reservoirs(id), PRIMARY KEY (grow_cycle_id, reservoir_id));

CREATE TABLE plant_positions (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  zone_id UUID NOT NULL REFERENCES zones(id),
  code TEXT NOT NULL,
  scene_binding TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (zone_id, code)
);

CREATE TABLE grow_cycle_positions (
  grow_cycle_id UUID NOT NULL REFERENCES grow_cycles(id),
  plant_position_id UUID NOT NULL REFERENCES plant_positions(id),
  occupied_from TIMESTAMPTZ NOT NULL,
  occupied_to TIMESTAMPTZ,
  PRIMARY KEY (grow_cycle_id, plant_position_id, occupied_from)
);

CREATE TABLE grow_cycle_stage_history (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  grow_cycle_id UUID NOT NULL REFERENCES grow_cycles(id),
  stage_key TEXT NOT NULL,
  started_at TIMESTAMPTZ NOT NULL,
  ended_at TIMESTAMPTZ,
  actor_id UUID REFERENCES users(id),
  CHECK (ended_at IS NULL OR ended_at >= started_at)
);

CREATE TABLE devices (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  facility_id UUID NOT NULL REFERENCES facilities(id),
  zone_id UUID REFERENCES zones(id),
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'provisioned' CHECK (status IN ('provisioned', 'online', 'offline', 'fault', 'decommissioned')),
  firmware_version TEXT,
  active_config_version TEXT,
  last_heartbeat TIMESTAMPTZ,
  metadata JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE device_channels (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  facility_id UUID NOT NULL REFERENCES facilities(id),
  entity_type TEXT NOT NULL,
  entity_id UUID NOT NULL,
  key TEXT NOT NULL,
  name TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('measurement', 'state', 'command', 'counter')),
  value_type TEXT NOT NULL CHECK (value_type IN ('number', 'boolean', 'enum')),
  unit TEXT,
  dimension TEXT,
  physical_minimum NUMERIC,
  physical_maximum NUMERIC,
  safe_minimum NUMERIC,
  safe_maximum NUMERIC,
  stale_after INTERVAL NOT NULL,
  UNIQUE (facility_id, key)
);

CREATE TABLE device_bindings (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  channel_id UUID NOT NULL REFERENCES device_channels(id),
  device_id UUID NOT NULL REFERENCES devices(id),
  physical_port TEXT,
  valid_from TIMESTAMPTZ NOT NULL,
  valid_to TIMESTAMPTZ,
  CHECK (valid_to IS NULL OR valid_to > valid_from)
);
CREATE UNIQUE INDEX ux_device_bindings_active ON device_bindings(channel_id) WHERE valid_to IS NULL;

CREATE TABLE measurements (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  channel_id UUID NOT NULL REFERENCES device_channels(id),
  observed_at TIMESTAMPTZ NOT NULL,
  received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  sequence BIGINT,
  value NUMERIC NOT NULL,
  unit TEXT NOT NULL,
  quality TEXT NOT NULL CHECK (quality IN ('good', 'suspect', 'stale', 'calibrating', 'fault', 'unknown')),
  source_device_id UUID REFERENCES devices(id),
  UNIQUE NULLS NOT DISTINCT (channel_id, observed_at, sequence)
);
CREATE INDEX ix_measurements_channel_time ON measurements(channel_id, observed_at DESC);
CREATE INDEX ix_measurements_observed_at ON measurements(observed_at);

CREATE TABLE latest_measurements (
  channel_id UUID PRIMARY KEY REFERENCES device_channels(id),
  measurement_id UUID NOT NULL REFERENCES measurements(id),
  observed_at TIMESTAMPTZ NOT NULL,
  value NUMERIC NOT NULL,
  unit TEXT NOT NULL,
  quality TEXT NOT NULL
);

CREATE TABLE farm_events (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  facility_id UUID NOT NULL REFERENCES facilities(id),
  type TEXT NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL,
  recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  actor_id UUID REFERENCES users(id),
  actor_label TEXT,
  summary TEXT NOT NULL,
  notes TEXT,
  correlation_id UUID
);
CREATE INDEX ix_farm_events_facility_time ON farm_events(facility_id, occurred_at DESC, id DESC);

CREATE TABLE farm_event_entities (
  event_id UUID NOT NULL REFERENCES farm_events(id),
  entity_type TEXT NOT NULL,
  entity_id UUID NOT NULL,
  PRIMARY KEY (event_id, entity_type, entity_id)
);

CREATE TABLE farm_event_quantities (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  event_id UUID NOT NULL REFERENCES farm_events(id),
  value NUMERIC NOT NULL,
  unit TEXT NOT NULL,
  dimension TEXT NOT NULL,
  material TEXT
);

CREATE TABLE media_objects (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  facility_id UUID NOT NULL REFERENCES facilities(id),
  storage_key TEXT NOT NULL UNIQUE,
  filename TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
  sha256 TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE observations (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  grow_cycle_id UUID NOT NULL REFERENCES grow_cycles(id),
  target_type TEXT NOT NULL,
  target_id UUID NOT NULL,
  category TEXT NOT NULL,
  severity TEXT NOT NULL CHECK (severity IN ('info', 'warning', 'critical')),
  notes TEXT NOT NULL,
  observed_at TIMESTAMPTZ NOT NULL,
  actor_id UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE observation_media (observation_id UUID NOT NULL REFERENCES observations(id), media_id UUID NOT NULL REFERENCES media_objects(id), PRIMARY KEY (observation_id, media_id));

CREATE TABLE inventory_items (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  facility_id UUID NOT NULL REFERENCES facilities(id),
  name TEXT NOT NULL,
  unit TEXT NOT NULL,
  reorder_level NUMERIC,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  UNIQUE (facility_id, name)
);

CREATE TABLE inventory_adjustments (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  item_id UUID NOT NULL REFERENCES inventory_items(id),
  event_id UUID REFERENCES farm_events(id),
  occurred_at TIMESTAMPTZ NOT NULL,
  quantity NUMERIC NOT NULL CHECK (quantity <> 0),
  unit TEXT NOT NULL,
  reason TEXT NOT NULL CHECK (reason IN ('purchase', 'use', 'correction', 'waste')),
  actor_id UUID REFERENCES users(id)
);

CREATE TABLE harvests (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  grow_cycle_id UUID NOT NULL REFERENCES grow_cycles(id),
  harvested_at TIMESTAMPTZ NOT NULL,
  mass_g NUMERIC NOT NULL CHECK (mass_g >= 0),
  waste_g NUMERIC NOT NULL DEFAULT 0 CHECK (waste_g >= 0),
  quality_notes TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE automation_rules (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  facility_id UUID NOT NULL REFERENCES facilities(id),
  zone_id UUID REFERENCES zones(id),
  name TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  current_version INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE automation_rule_versions (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  rule_id UUID NOT NULL REFERENCES automation_rules(id),
  version INTEGER NOT NULL,
  mode TEXT NOT NULL CHECK (mode IN ('observe', 'simulate', 'automatic')),
  definition JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (rule_id, version)
);

CREATE TABLE alert_definitions (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  facility_id UUID NOT NULL REFERENCES facilities(id),
  key TEXT NOT NULL,
  severity TEXT NOT NULL CHECK (severity IN ('warning', 'critical')),
  definition JSONB NOT NULL,
  UNIQUE (facility_id, key)
);

CREATE TABLE alerts (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  definition_id UUID NOT NULL REFERENCES alert_definitions(id),
  entity_type TEXT NOT NULL,
  entity_id UUID NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('open', 'acknowledged', 'resolved')),
  title TEXT NOT NULL,
  detail TEXT NOT NULL,
  opened_at TIMESTAMPTZ NOT NULL,
  acknowledged_at TIMESTAMPTZ,
  acknowledged_by UUID REFERENCES users(id),
  resolved_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX ux_alerts_active_condition ON alerts(definition_id, entity_type, entity_id) WHERE status <> 'resolved';

CREATE TABLE alert_events (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  alert_id UUID NOT NULL REFERENCES alerts(id),
  transition TEXT NOT NULL CHECK (transition IN ('opened', 'acknowledged', 'updated', 'resolved')),
  occurred_at TIMESTAMPTZ NOT NULL,
  actor_id UUID REFERENCES users(id),
  detail JSONB NOT NULL DEFAULT '{}'
);

CREATE TABLE commands (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  target_channel_id UUID NOT NULL REFERENCES device_channels(id),
  requested_by UUID REFERENCES users(id),
  command_type TEXT NOT NULL,
  value JSONB NOT NULL,
  reason TEXT NOT NULL,
  state TEXT NOT NULL CHECK (state IN ('pending', 'published', 'acknowledged', 'applied', 'rejected', 'timed_out', 'cancelled')),
  idempotency_key TEXT,
  requested_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  rejection_code TEXT,
  correlation_id UUID
);
CREATE UNIQUE INDEX ux_commands_idempotency ON commands(idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE TABLE command_attempts (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  command_id UUID NOT NULL REFERENCES commands(id),
  attempt INTEGER NOT NULL,
  published_at TIMESTAMPTZ,
  acknowledged_at TIMESTAMPTZ,
  result TEXT,
  detail JSONB NOT NULL DEFAULT '{}',
  UNIQUE (command_id, attempt)
);

CREATE TABLE actuation_events (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  command_id UUID NOT NULL REFERENCES commands(id),
  started_at TIMESTAMPTZ NOT NULL,
  ended_at TIMESTAMPTZ,
  requested_value JSONB NOT NULL,
  applied_value JSONB,
  CHECK (ended_at IS NULL OR ended_at >= started_at)
);

CREATE TABLE manual_overrides (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  target_channel_id UUID NOT NULL REFERENCES device_channels(id),
  value JSONB NOT NULL,
  reason TEXT NOT NULL,
  started_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  actor_id UUID REFERENCES users(id),
  CHECK (expires_at > started_at)
);

CREATE TABLE scene_layouts (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  facility_id UUID NOT NULL REFERENCES facilities(id),
  name TEXT NOT NULL,
  camera_position JSONB NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  UNIQUE (facility_id, name)
);

CREATE TABLE scene_entities (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  scene_id UUID NOT NULL REFERENCES scene_layouts(id),
  entity_type TEXT NOT NULL,
  entity_id UUID NOT NULL,
  asset_key TEXT,
  transform JSONB NOT NULL,
  interaction_profile TEXT NOT NULL,
  parent_scene_entity_id UUID REFERENCES scene_entities(id),
  UNIQUE (scene_id, entity_type, entity_id)
);

CREATE TABLE outbox_messages (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  topic TEXT NOT NULL,
  message_key TEXT NOT NULL,
  payload JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  published_at TIMESTAMPTZ,
  attempts INTEGER NOT NULL DEFAULT 0,
  last_error TEXT
);
CREATE INDEX ix_outbox_pending ON outbox_messages(created_at) WHERE published_at IS NULL;

CREATE TABLE browser_compatible_states (
  singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
  state JSONB NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE audit_log (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  actor_id UUID REFERENCES users(id),
  action TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id UUID,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  correlation_id UUID,
  detail JSONB NOT NULL DEFAULT '{}'
);

CREATE TRIGGER facilities_set_updated_at BEFORE UPDATE ON facilities FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER zones_set_updated_at BEFORE UPDATE ON zones FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER reservoirs_set_updated_at BEFORE UPDATE ON reservoirs FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER grow_cycles_set_updated_at BEFORE UPDATE ON grow_cycles FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER devices_set_updated_at BEFORE UPDATE ON devices FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER automation_rules_set_updated_at BEFORE UPDATE ON automation_rules FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'grownerve_app') THEN
    GRANT USAGE ON SCHEMA public TO grownerve_app;
    GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA public TO grownerve_app;
    GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO grownerve_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE ON TABLES TO grownerve_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO grownerve_app;
  END IF;
END
$$;

INSERT INTO schema_migrations(version) VALUES (1);
