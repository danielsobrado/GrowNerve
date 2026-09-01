DROP TABLE IF EXISTS audit_log, browser_compatible_states, outbox_messages, scene_entities, scene_layouts,
  manual_overrides, actuation_events, command_attempts, commands, alert_events, alerts, alert_definitions,
  automation_rule_versions, automation_rules, harvests, inventory_adjustments, inventory_items,
  observation_media, observations, media_objects, farm_event_quantities, farm_event_entities, farm_events,
  latest_measurements, measurements, device_bindings, device_channels, devices, grow_cycle_stage_history,
  grow_cycle_positions, plant_positions, grow_cycle_reservoirs, grow_cycle_zones, grow_cycles,
  stage_schedules, stage_setpoints, recipe_stages, grow_recipe_versions, grow_recipes, reservoirs,
  varieties, crops, zones, facilities, users, schema_migrations CASCADE;
DROP FUNCTION IF EXISTS set_updated_at();
DROP FUNCTION IF EXISTS uuidv7();
