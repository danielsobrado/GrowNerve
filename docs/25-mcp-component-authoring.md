# 25 — MCP Component Authoring and Farm Editing

## Purpose

GrowNerve should expose component and farm-layout authoring through MCP so an AI agent can create, inspect, validate, place, and connect components without understanding Three.js internals or editing repository files directly.

The MCP layer is an adapter over GrowNerve application services.

The core rule is:

```text
MCP does not manipulate Three.js.
MCP does not bypass validation.
MCP does not write arbitrary files as its domain API.
MCP calls the same component/farm services used by the normal product UI.
```

This gives AI-assisted authoring the same invariants, permissions, safety checks, versioning, and import/export behavior as human-driven workflows.

See `24-component-plugin-system.md` for the component model.

## Goals

MCP should make workflows such as these possible:

- "Create a generic 30 L rectangular reservoir, 60 x 40 x 25 cm."
- "Add a pH probe to the reservoir and bind it to the reservoir pH channel."
- "Create a reusable aeration assembly from an air pump, tubing, and two air stones."
- "List every component with the `calibration` capability."
- "Validate this community component pack before I import it."
- "Place four net pots evenly across the reservoir lid."
- "Show me missing component dependencies in this farm archive."
- "Clone this LED definition and make a generic 240 W version using primitive geometry until I provide a GLB."

## Non-goals

V0 MCP is not:

- a direct filesystem editor
- a code generator that injects JavaScript into the browser
- a way to bypass component-pack validation
- a way to bypass domain authorization or actuator safety
- a general 3D modeling replacement for Blender/CAD tools
- a remote-control protocol for raw MQTT topics

## Architecture

```text
          ChatGPT / agent / MCP client
                     |
                     v
              GrowNerve MCP
                     |
                     v
          Application service layer
          /          |           \
         v           v            v
 Component       Farm layout     Import/export
 services        services        services
         \           |            /
          +----------+-----------+
                     |
                     v
       domain validation / registry
                     |
          +----------+-----------+
          |                      |
          v                      v
      server mode            browser/tooling
```

The MCP adapter should depend on stable application interfaces. It should not call HTTP endpoints internally merely because the user-facing application also has an HTTP API.

Where GrowNerve is deployed as a remote service, an MCP transport process may call the server API, but the exposed MCP contract must remain domain-oriented.

## Safety boundary

Component authoring and farm editing are configuration operations. Physical actuator commands remain separate.

If future MCP tools expose real actuator command intent, they must go through the normal command workflow:

```text
MCP command request
 -> authentication/authorization
 -> domain validation
 -> safety/interlock validation
 -> durable command record
 -> device transport
 -> acknowledgement/state telemetry
```

An MCP caller never gets a privileged path around the existing control model.

## Tool design rules

MCP tools should be:

- small and composable
- domain named
- deterministic where possible
- explicit about dry-run versus mutation
- version-aware
- safe to retry when documented as idempotent
- structured enough that agents do not need to parse free-form prose

Avoid one giant tool such as:

```text
edit_grownerve_project(instructions: string)
```

Prefer explicit operations that expose validation failures as structured results.

## Initial component tools

### `components.list`

Purpose:

List available component definitions with optional filters.

Filters may include:

- category
- tag
- capability
- namespace/pack
- model type

Return only summary metadata unless detailed definitions are requested separately.

### `components.get`

Purpose:

Return one normalized component definition by stable ID and version.

Inputs:

```text
id
version | resolved version
```

Output includes the content hash used for conflict/reproducibility checks.

### `components.search`

Purpose:

Search component metadata by human terms such as "water level sensor" or "240 W LED".

Search is over installed/available registry metadata. A future remote catalog may be an additional explicitly selected source.

### `components.schema`

Purpose:

Return the current authoring schema or a focused sub-schema.

Examples:

```text
component
plugin
primitive-model
port
anchor
assembly
```

This helps an AI author valid JSON without embedding schema knowledge into the MCP client.

### `components.validate`

Purpose:

Validate an uninstalled component definition or pack.

Validation levels:

```text
schema
semantic
references
assets
all
```

Return structured diagnostics:

```json
{
  "valid": false,
  "errors": [
    {
      "code": "COMPONENT_DIMENSION_INVALID",
      "path": "/dimensions/height",
      "message": "height must be greater than zero"
    }
  ],
  "warnings": []
}
```

Do not return only a prose blob.

### `components.create`

Purpose:

Create a new local component definition through the application service.

V0 supports declarative content only.

Expected use:

1. caller gets schema if needed
2. caller submits candidate definition
3. service validates
4. service stores definition only if valid
5. result returns stable ID, version, and content hash

The service must reject an attempt to overwrite an existing `id` + `version` with different content.

### `components.clone`

Purpose:

Create a new component definition using another definition as a starting point.

The caller must provide a new stable ID or allowed local namespace identity.

Cloning must not silently preserve vendor branding/license metadata that is not appropriate for the new definition.

### `components.update`

Purpose:

Create a new component version from an existing local component.

Published/imported immutable versions are not edited in place.

Conceptual behavior:

```text
1.0.0 -> draft changes -> 1.0.1 / 1.1.0 / 2.0.0
```

The service may provide compatibility guidance but must not invent the version bump automatically without caller intent in V0.

### `components.delete`

Purpose:

Delete an unpublished/local definition version when nothing depends on it.

Reject deletion if a farm/assembly references it unless the caller first migrates/removes those references.

### `components.install_pack`

Purpose:

Validate and install a component pack.

The MCP tool accepts a file/reference supplied by the host environment; it does not fetch arbitrary Internet URLs by default.

Return:

- pack ID/version
- installed component IDs/versions
- warnings
- resolved/conflicting dependencies

### `components.export_pack`

Purpose:

Create a portable ZIP for one local pack or selected local definitions and assets.

## Asset tools

Assets need explicit tools because binary/model validation is separate from JSON authoring.

### `assets.attach_model`

Attach a supplied GLB/glTF asset to a local draft component.

Requirements:

- content/type validation
- size limits
- path normalization
- model inspection before activation

### `assets.attach_thumbnail`

Attach a supported image thumbnail.

### `assets.inspect_model`

Return useful model metadata without requiring the MCP client to parse GLB:

- bounding dimensions
- node/mesh count
- triangle count where available
- material count
- texture count/sizes
- animation names
- detected unit/pivot warnings where inferable

### `assets.validate_model`

Validate against GrowNerve runtime budgets/policy.

Validation warnings may include:

- oversized geometry
- excessive texture dimensions
- unexpected external references
- unsupported extension
- implausible dimensions
- pivot/origin far outside bounds

## Primitive-model authoring

Primitive geometry is essential for MCP because an AI should be able to create useful components before an authored GLB exists.

Example user request:

```text
Create a black 30 L DWC reservoir, 60 x 40 x 25 cm.
```

Possible MCP sequence:

```text
components.schema(primitive-model)
components.create(... box primitive ...)
components.validate(...)
```

The renderer owns how a primitive becomes Three.js geometry.

MCP only supplies validated primitive parameters.

## Farm-layout tools

### `farms.get_layout`

Return the normalized layout for a farm or scene.

### `farms.validate_layout`

Validate:

- component dependencies
- transforms
- domain bindings
- assemblies
- connections
- referenced ports/anchors

### `farms.add_component`

Inputs conceptually include:

```text
farmId
component id/version
instance id or generated id
transform
configuration
bindings
```

The service validates the definition exists before writing the instance.

### `farms.remove_component`

Reject removal when unresolved connection/assembly dependencies would be left behind unless the request explicitly includes a valid cascading operation supported by the service.

### `farms.move_component`

Updates only transform/placement state.

Do not route movement through component-definition update APIs.

### `farms.configure_component`

Updates allowed instance configuration and bindings.

Generic definition metadata remains immutable.

### `farms.connect_components`

Inputs identify source/target instance and port IDs.

Validation checks compatibility before committing the connection.

### `farms.disconnect_components`

Removes an existing connection by stable connection ID.

### `farms.place_on_anchor`

Later tool for snap/placement workflows.

The service resolves compatible anchors and computes/stores an explicit resulting transform. The persisted farm remains readable without replaying hidden snap logic.

### `farms.list_missing_dependencies`

Useful after import or when opening an older project.

Returns exact missing component IDs/versions/hashes.

## Assembly tools

### `assemblies.create`

Create a reusable assembly from component instances and internal connections.

### `assemblies.get`

Return the normalized assembly definition.

### `assemblies.instantiate`

Place a reusable assembly into a farm with a root transform.

### `assemblies.validate`

Checks:

- component dependencies
- cycles
- connection validity
- duplicate child IDs
- transform validity

## Import/export tools

### `projects.validate_import`

Performs non-destructive validation of a `.grownerve.json` or bundled archive.

### `projects.import`

Runs the same transactional import path as the product UI.

MCP must not have a separate "force" path that skips validation.

### `projects.export`

Exports the normal portable farm archive and, when requested, a bundled archive containing required local component assets.

## Suggested MCP resources

In addition to invokable tools, MCP resources can expose read-only material useful to agents:

```text
grownerve://schemas/component/current
grownerve://schemas/plugin/current
grownerve://schemas/farm-layout/current
grownerve://registry/components
grownerve://farm/{id}/layout
grownerve://docs/component-authoring
```

Resources should not be the only way to mutate state.

## Example workflow — create a pH sensor

User:

```text
Create a generic pH probe component with a 0-14 pH telemetry channel and a calibrate action.
```

Expected agent flow:

```text
components.schema(component)
       |
       v
construct candidate definition
       |
       v
components.validate(candidate)
       |
       +-- invalid -> repair candidate -> validate again
       |
       v
components.create(candidate)
       |
       v
return ID/version/hash
```

No Three.js code is generated.

## Example workflow — add a sensor to a real reservoir

User:

```text
Add the generic pH probe to reservoir R1 and bind it to the reservoir pH channel.
```

Expected flow:

```text
components.search("generic pH")
farms.get_layout(farm)
resolve reservoir instance/domain binding
resolve logical pH channel
farms.add_component(...)
farms.configure_component(bindings...)
farms.validate_layout(farm)
```

If a physical placement anchor exists, the agent may additionally call `farms.place_on_anchor`.

## Example workflow — create an aeration assembly

User:

```text
Make an assembly with one air pump, two air stones and tubing so I can reuse it in another reservoir.
```

Expected flow:

```text
components.search("air pump")
components.search("air stone")
components.search("tube")
assemblies.create(children + transforms + connections)
assemblies.validate(...)
```

The assembly references existing definitions; it does not duplicate them.

## Example workflow — missing vendor component

User:

```text
Add my new DFRobot EC sensor.
```

MCP should first search the installed registry.

```text
components.search("DFRobot EC")
```

If not found, the agent can create a local definition from information the user provides or from independently retrieved authoritative product data when the host environment permits web research.

The MCP server itself should not silently scrape the web.

## Validation and error model

Use stable machine-readable error codes.

Examples:

```text
COMPONENT_NOT_FOUND
COMPONENT_VERSION_CONFLICT
COMPONENT_SCHEMA_INVALID
COMPONENT_SEMANTIC_INVALID
PACK_ASSET_MISSING
PACK_PATH_INVALID
PACK_TOO_LARGE
MODEL_FORMAT_UNSUPPORTED
MODEL_BUDGET_EXCEEDED
PORT_NOT_FOUND
PORT_INCOMPATIBLE
ANCHOR_NOT_FOUND
ASSEMBLY_CYCLE
FARM_LAYOUT_INVALID
FARM_DEPENDENCY_MISSING
DOMAIN_BINDING_INVALID
PERMISSION_DENIED
SAFETY_REJECTED
```

Each error may include:

```text
code
message
path/entity id
details
recoverable flag
```

Agents should not need to string-match human error text.

## Idempotency and retries

Read operations are naturally retryable.

Mutation tools should support safe retry where useful.

Preferred techniques:

- caller-provided request/idempotency key for create/install/import operations
- stable caller-provided instance IDs where appropriate
- optimistic version checks for farm-layout mutation
- immutable component versions

Do not make `create` return a different component every time the same transport request is retried.

## Concurrency

Farm-layout writes follow the same optimistic concurrency model as other farm state.

A tool mutating a farm should accept/return the relevant farm/layout version or equivalent concurrency token.

On conflict:

```text
FARM_VERSION_CONFLICT
```

The agent must re-read and reconcile instead of silently overwriting another user's work.

## Permissions

MCP authorization is capability-based at the application boundary.

Example permission classes:

```text
components.read
components.author
components.install
farm.read
farm.layout.edit
farm.binding.edit
project.import
project.export
```

If future real actuator tools exist, they use the existing command permissions and safety checks instead of component-authoring permissions.

## Auditability

Meaningful MCP mutations should record actor/source metadata such as:

```text
actor identity
source = mcp
tool name
request/idempotency ID
timestamp
affected farm/component IDs
result
```

Do not log full binary assets or secrets.

## Browser-only implications

A static GitHub Pages deployment cannot itself expose a normal network-listening MCP server from the browser.

Therefore MCP has three practical deployment modes:

### Full runtime MCP

A server-side MCP adapter uses GrowNerve application services directly.

### Local tooling MCP

A developer/desktop MCP process reads/writes GrowNerve portable project/component packs through the same schemas and validators, then the user imports the resulting archive into browser mode.

### Future browser bridge

A browser extension/desktop companion could bridge to an open GrowNerve browser session later, but this is not required for V0.

The component file formats are deliberately independent of MCP so static hosting is never dependent on an agent connection.

## Security

MCP must not weaken the declarative plugin security model.

Rules:

- no arbitrary script injection into packs
- no arbitrary filesystem paths
- no arbitrary remote URL fetch by the GrowNerve MCP server
- validate every supplied archive/asset
- enforce pack/model size limits
- normalize and constrain paths
- do not allow an MCP caller to register a component by directly editing registry storage
- imported Markdown/text is treated as untrusted content
- component metadata cannot grant permissions

## Observability

Record metrics/logs for:

- tool invocation count/result
- validation failure codes
- component install/import failures
- model validation duration
- pack sizes
- farm-layout conflicts

Do not record user prompts as a requirement for normal operational metrics.

## Implementation plan

### M1 — Read-only MCP

Deliver:

- `components.list`
- `components.get`
- `components.search`
- `components.schema`
- `farms.get_layout`
- validation resources/docs

Exit criteria:

- an MCP client can understand the installed component model without repo access

### M2 — Validation and local authoring

Deliver:

- `components.validate`
- `components.create`
- `components.clone`
- versioned `components.update`
- primitive model authoring

Exit criteria:

- an agent can create a valid primitive component entirely through MCP

### M3 — Asset authoring

Deliver:

- attach/inspect/validate model
- attach thumbnail
- pack export

Exit criteria:

- an agent can combine supplied GLB assets with a valid definition without direct filesystem editing

### M4 — Farm layout editing

Deliver:

- add/remove/move/configure component
- connect/disconnect
- layout validation
- concurrency handling

Exit criteria:

- an agent can build the reference pilot layout using registry components

### M5 — Assemblies

Deliver:

- create/get/validate/instantiate assembly

Exit criteria:

- reusable DWC/aeration assemblies can be authored and placed through MCP

### M6 — Project import/export

Deliver:

- validate import
- transactional import
- standard/bundled export

### M7 — Optional community catalog integration

Only after a component catalog exists.

MCP may search catalog metadata and request an explicit install through normal pack validation. It must not auto-install arbitrary Internet content without user intent and policy checks.

## Acceptance criteria

The MCP design is successful when all of the following are true:

- an agent can create a component without generating Three.js code
- the same candidate is accepted/rejected identically through UI/API/MCP validation paths
- a primitive placeholder can later be replaced by a GLB through a new version without changing farm domain identity
- farm edits remain valid after browser/server export/import
- port incompatibilities are rejected before persistence
- component versions are immutable and reproducible
- no MCP operation can install executable plugin code in V0
- no MCP layout operation bypasses farm concurrency rules
- no MCP command path bypasses authorization/safety

## Recommended first implementation

Do not start by building a large MCP surface.

The highest-value proof is:

```text
components.schema
components.search
components.validate
components.create
farms.get_layout
farms.add_component
farms.validate_layout
```

With primitive geometry, those tools are enough to prove that an AI can create and place a new component while GrowNerve remains renderer-agnostic and safe.
