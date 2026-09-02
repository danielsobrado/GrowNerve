# 25 — MCP Component Authoring and Farm Editing

## Purpose

GrowNerve should expose component authoring and farm-layout editing through MCP so an AI agent can create, validate, place, and connect components without generating Three.js code or editing repository files directly.

The MCP server is an adapter over GrowNerve application/domain services.

```text
MCP tool call
   -> authentication / authorization
   -> component or farm service
   -> validation / compare-and-swap
   -> normal persistence
```

MCP does not own a second data model and does not get a privileged write path.

See `24-component-plugin-system.md` for the component contract and the migration from the current profile-based twin.

## Current code baseline

There is no MCP server in the repository today.

The implementation should fit the architecture that already exists:

- the authoritative full runtime is Go.
- `cmd/api` is the current server entry point.
- farm configuration is one versioned JSON document protected by compare-and-swap.
- `internal/farm` owns the state-write/concurrency boundary.
- `internal/farm/authz.go` already separates viewer/operator/manager/administrator authorization.
- command safety is separate from authorization.
- browser mode uses the same `FarmData` model through a Dexie repository.
- the frontend's server adapter talks to the HTTP API; it does not expose internal Go services.
- the component registry described in document 24 has not been implemented yet.

Therefore MCP should be added **after** component/layout services exist, not before them.

## Protocol baseline

Target MCP protocol revision **2026-07-28**.

At the time of this document, that revision is the current stable MCP specification and the official Go SDK supports it in the v1.7+ line.

Relevant upstream references:

- <https://modelcontextprotocol.io/specification/2026-07-28>
- <https://github.com/modelcontextprotocol/go-sdk>

Important consequences for GrowNerve:

- the 2026-07-28 core protocol is stateless; do not design application correctness around MCP transport sessions.
- every cross-call workflow state must be represented by explicit GrowNerve IDs/handles.
- tool input/output schemas use JSON Schema 2020-12.
- tool/resource listings should be deterministic so clients can cache them reliably.
- deprecated MCP roots/sampling/logging features are not needed for this integration.
- use normal application logging/OpenTelemetry conventions rather than MCP's deprecated logging feature.

If GrowNerve later targets a newer MCP revision, update this document/ADR deliberately rather than silently changing semantics.

## Recommended deployment shape

### First implementation: integrated Go HTTP MCP endpoint

Prefer one MCP package inside the current Go modular monolith and expose it through the existing API process:

```text
cmd/api
  |
  +-- normal GrowNerve HTTP API
  |
  +-- /mcp
        |
        v
   internal/mcpserver
        |
        +-- component services
        +-- farm/layout services
        +-- existing auth/audit boundaries
```

Why this is the best first shape:

- reuses the existing Go runtime instead of adding a Node sidecar.
- reuses the same configuration, logging, database pool, authentication, and authorization infrastructure.
- can mutate farm state through the same compare-and-swap path.
- avoids one service calling another service over localhost HTTP just to reach its own domain logic.
- can still be used by local MCP clients against `http://localhost:<port>/mcp`.

### Do not start with a separate stdio daemon

A standalone stdio process would either duplicate storage/business logic or call the API as a second client. Neither is needed for the first proof.

A stdio adapter may be added later for tooling if a concrete host requires it. It should wrap the same `internal/mcpserver` handlers/services, not reimplement them.

### Browser-only runtime

A static GitHub Pages application cannot host a normal network-listening MCP server.

Browser-only mode therefore remains independent from MCP. Agents can create portable component packs/project archives through the full/local Go runtime and the browser can import them through the normal validated import path.

Do not make browser-only usability depend on an MCP connection.

## Security boundary

MCP is another authenticated client, not an administrator shortcut.

The same separation remains mandatory:

```text
authorization: may this caller request this operation?
safety:        is this physical operation safe right now?
```

Component/layout authoring is configuration editing. The initial role mapping should reuse existing actions instead of inventing a parallel permission system:

```text
read component/layout state       -> viewer / farm.read
mutate farm layout bindings       -> manager / farm.write_state
install global component packs    -> administrator / system.administer
issue low-risk actuator command   -> operator / command.issue + safety validation
```

Only create finer-grained actions such as `component.install` when a real authorization need appears.

If future MCP tools expose physical commands, they call the existing command service and retain durable delivery, expiry, idempotency, interlocks, and acknowledgement semantics.

## Tool design rules

Tools should be:

- narrow and composable
- deterministic where practical
- closed-domain by default
- explicit about exact component revision and farm version
- structured on input and output
- safe to retry only when their contract says so
- consistent with existing domain naming

Avoid a giant prompt-shaped mutation tool such as:

```text
edit_grownerve_project(instructions: string)
```

That would move validation and intent parsing into an opaque mutation boundary and make concurrency/audit behavior difficult to reason about.

## MCP schema rules

For every tool:

- define `inputSchema` and `outputSchema` using JSON Schema 2020-12.
- use an object input root.
- default contract objects to `additionalProperties: false`.
- use enums for known operation modes and error codes.
- return structured results; human-readable text is supplementary.
- keep schema depth bounded and do not dereference untrusted external `$ref` URIs.

Tool-level validation is not a replacement for domain validation. The handler validates again using the normal component/farm services.

## Tool annotations

Use standard MCP annotations accurately:

```text
readOnlyHint
destructiveHint
idempotentHint
openWorldHint
```

Examples:

```text
components.list
  readOnlyHint: true
  openWorldHint: false

components.create
  readOnlyHint: false
  destructiveHint: false
  idempotentHint: true only when exact revision/digest semantics make retries safe
  openWorldHint: false

components.install_pack
  readOnlyHint: false
  destructiveHint: false
  openWorldHint: false
```

These are client hints, not security controls. GrowNerve authorization/validation remains authoritative even if a client ignores annotations.

## Tool names

Dot-separated names are valid in the current MCP tool-name guidance and read well for this domain.

Keep them stable once published:

```text
components.list
components.get
components.validate
components.create
farms.get_layout
farms.set_component
farms.validate_layout
projects.validate_import
```

Do not rename tools casually; agents and cached tool catalogs may depend on them.

## Minimal V0 tool surface

The first MCP proof should remain small enough to test exhaustively.

### `components.list`

Read-only list/search over the installed registry.

Inputs may include:

```text
query
category
capability
pack_id
limit
cursor
```

Return compact summaries in deterministic order.

Do not mix remote marketplace search into this tool in V0.

### `components.get`

Return one exact normalized revision.

Input:

```json
{
  "component_id": "grownerve.sensor.ph.generic",
  "version": "1.0.0",
  "digest": "sha256:..."
}
```

If digest is omitted for an interactive lookup, the server may return available exact revisions, but a farm write must always persist a resolved exact digest.

### `components.validate`

Validate a candidate component JSON without storing it.

Output shape:

```json
{
  "valid": false,
  "errors": [
    {
      "code": "COMPONENT_SCHEMA_INVALID",
      "path": "/model/parameters/height_m",
      "message": "height_m must be greater than zero"
    }
  ],
  "warnings": []
}
```

Validation uses the same schema fixtures and semantic validator as browser/server import.

### `components.create`

Create one immutable local component revision.

V0 scope is deliberately limited to definitions that use primitive geometry and existing local assets already known to the component service.

Inputs include the complete candidate definition and an optional caller `request_id`.

Idempotency rule:

```text
same component_id + version + digest -> return existing revision
same component_id + version + different digest -> conflict
```

This makes network retries safe without a hidden MCP session.

### `farms.get_layout`

Return:

```text
farm version
scene layout
resolved component refs
missing dependencies
```

The `farm_version` is the existing farm-state concurrency token, not a new MCP-only version.

### `farms.set_component`

Upsert the component reference/configuration for an existing operational scene entity identified by:

```text
entity_type
entity_id
```

This intentionally matches the current `SceneEntity` identity model.

Inputs conceptually include:

```json
{
  "farm_version": 42,
  "layout_id": "uuid",
  "entity_type": "device",
  "entity_id": "uuid",
  "component_ref": {
    "component_id": "grownerve.air.circulation-fan.generic",
    "version": "1.0.0",
    "digest": "sha256:..."
  },
  "position": [0, 0, 0],
  "rotation": [0, 0, 0],
  "scale": [1, 1, 1],
  "channel_bindings": {}
}
```

The handler:

1. checks authorization.
2. loads the requested farm version.
3. validates component revision availability.
4. validates entity/domain references.
5. validates channel bindings.
6. writes through the existing compare-and-swap state boundary.
7. returns the new farm version.

Using an upsert against the existing entity key makes safe retries easier than creating arbitrary duplicate scene-instance IDs.

### `farms.validate_layout`

Read-only validation of the current or supplied candidate layout.

Checks:

- component dependencies
- exact revision digests
- entity references
- transform finiteness
- channel bindings
- ports/connections when implemented

## Tools to add only after V0 proves useful

Do not publish a large speculative surface at the start.

Later tools may include:

```text
components.install_pack
components.export_pack
components.create_revision
assets.inspect_model
assets.validate_model
farms.remove_component
farms.connect_components
farms.disconnect_components
assemblies.create
assemblies.instantiate
projects.validate_import
projects.import
projects.export
```

Each arrives with its storage/validation use case and tests.

## Pack installation

When pack installation is added, keep it separate from component authoring.

`components.install_pack`:

- accepts a host-supplied file/blob reference or already-uploaded GrowNerve asset handle.
- does not fetch arbitrary Internet URLs.
- runs all ZIP/path/size/digest/JSON/model validation from document 24.
- commits the pack atomically only after the complete pack validates.
- requires administrator-level authorization initially.

If the pack is already installed with the same pack revision/digest, return success idempotently.

## Model assets and stateless MCP

Do not create implicit "current draft" state tied to an MCP connection. The current protocol is stateless at its core.

When multi-step asset authoring is eventually required, use an explicit GrowNerve draft ID:

```text
components.begin_draft -> draft_id
assets.attach_model(draft_id, ...)
components.validate_draft(draft_id)
components.publish_draft(draft_id, version)
```

The draft is normal GrowNerve application state with ownership, expiry, audit records, and cleanup. It is not an MCP session variable.

Primitive-only V0 avoids needing this complexity.

## Resources

Use MCP resources for read-only material that is useful across many calls.

Suggested resources:

```text
grownerve://schemas/component/1
grownerve://schemas/pack/1
grownerve://docs/component-authoring
grownerve://registry/components/<component-id>/<version>/<digest>
```

Avoid using one enormous `grownerve://registry/components` document when the catalog grows. List/search tools are a better paginated interface.

Schemas/resources are versioned and deterministic so clients can cache them.

## Structured results and errors

Tool validation/domain failures should return an MCP tool result marked as an error with structured error content, not misuse transport/protocol errors for ordinary business rejection.

Use stable GrowNerve error codes such as:

```text
COMPONENT_NOT_FOUND
COMPONENT_VERSION_CONFLICT
COMPONENT_DIGEST_MISMATCH
COMPONENT_SCHEMA_INVALID
COMPONENT_SEMANTIC_INVALID
PACK_ASSET_MISSING
PACK_PATH_INVALID
PACK_TOO_LARGE
MODEL_FORMAT_UNSUPPORTED
MODEL_BUDGET_EXCEEDED
CHANNEL_BINDING_INVALID
PORT_NOT_FOUND
PORT_INCOMPATIBLE
FARM_LAYOUT_INVALID
FARM_DEPENDENCY_MISSING
FARM_VERSION_CONFLICT
PERMISSION_DENIED
SAFETY_REJECTED
```

Example output:

```json
{
  "ok": false,
  "error": {
    "code": "FARM_VERSION_CONFLICT",
    "message": "Farm state changed; read the current layout and retry",
    "recoverable": true,
    "current_farm_version": 43
  }
}
```

Agents must not need to parse English error strings to decide what to do next.

## Concurrency

GrowNerve already has the concurrency contract MCP needs.

The server farm store uses optimistic compare-and-swap. MCP layout writes reuse that exact version.

```text
farms.get_layout -> farm_version 42
farms.set_component(expected 42)
    |
    +-- success -> version 43
    |
    +-- conflict -> FARM_VERSION_CONFLICT
```

On conflict the agent re-reads and reconciles. It never force-writes through the conflict.

Do not add a second independent `layout_version` until layouts are persisted independently from the farm document.

## Idempotency

Different mutation classes use different existing invariants.

### Immutable component revision

Idempotent by exact identity/digest:

```text
(component_id, version, digest)
```

### Scene binding upsert

Idempotent when the same farm version/entity binding/request payload is applied once. A retried write after a successful version change receives a version conflict and must re-read rather than duplicate an entity.

### Physical command

Use the existing command idempotency/durable command path. MCP does not invent a second command system.

## Auditability

Meaningful MCP writes should extend the existing audit approach.

Record:

```text
actor subject
source = mcp
tool name
request_id when supplied
component/farm/entity identifiers
previous/new farm version where relevant
result/error code
timestamp
```

Do not log:

- bearer tokens
- secrets
- complete binary assets
- entire user prompts as a normal operational requirement

The audit record should describe the mutation, not capture the model's private reasoning.

## Authentication and remote MCP

The integrated `/mcp` endpoint uses the same production authentication posture as the rest of GrowNerve.

For remote deployments:

- require TLS at the normal reverse-proxy boundary.
- reuse current bearer/OIDC principal resolution where compatible with the SDK transport.
- authorize every tool call independently; do not assume prior calls established a trusted session.
- bind credentials to the configured issuer/provider and follow the current MCP/OAuth security guidance when native MCP authorization is introduced.

Do not enable anonymous remote MCP simply because the local development server is easy to reach.

## Open-world behavior

GrowNerve MCP is closed-domain by default.

The server itself should not:

- scrape vendor websites
- perform arbitrary web searches
- install from arbitrary URLs
- follow URLs found inside component metadata

If an MCP host has its own web-research capability, the agent may use that capability to gather authoritative product information and then submit a candidate definition to GrowNerve validation.

This keeps untrusted web retrieval outside the component registry trust boundary.

## Example — create a generic pH component

User intent:

```text
Create a generic pH probe with a 0-14 pH measurement slot.
```

Agent flow:

```text
read grownerve://schemas/component/1
        |
        v
construct primitive component definition
        |
        v
components.validate
        |
        +-- invalid -> repair -> validate
        |
        v
components.create
        |
        v
return exact component_id/version/digest
```

No Three.js source file is generated.

## Example — bind that component to an existing sensor entity

```text
components.list(query="pH probe")
farms.get_layout
resolve existing domain entity + channel UUID
farms.set_component(expected farm_version, component_ref, channel_bindings)
farms.validate_layout
```

If the farm changes between read and write, the last mutation returns `FARM_VERSION_CONFLICT`; the agent re-reads rather than overwriting another user's edit.

## Example — vendor component not installed

User intent:

```text
Add my DFRobot EC sensor.
```

GrowNerve MCP first searches only its installed registry:

```text
components.list(query="DFRobot EC")
```

If no exact definition exists, the agent may construct a local candidate from user-supplied or independently researched authoritative data and call `components.validate`/`components.create`.

The GrowNerve MCP server does not silently crawl the web.

## Testing

### Protocol tests

Use the official MCP Inspector and SDK test clients against the integrated `/mcp` endpoint.

Verify:

- current protocol negotiation/discovery
- deterministic `tools/list`
- tool schemas are valid JSON Schema 2020-12
- output conforms to each `outputSchema`
- annotations match real behavior
- read-only tools do not mutate state
- protocol/business errors are separated correctly

### Domain parity tests

For the same candidate payload, assert equivalent accept/reject behavior through:

```text
component validator directly
normal HTTP/UI import path
MCP tool
```

### Concurrency tests

Reuse the existing farm concurrency strategy:

- two MCP writers read the same farm version.
- one wins.
- the second receives a version conflict.
- no scene binding is silently lost.

### Authorization tests

At minimum:

```text
viewer  -> can read, cannot mutate
operator -> does not gain configuration-edit rights from MCP
manager -> can edit farm layout
admin   -> can install global packs when that tool exists
```

### Security tests

When pack tools arrive, cover malicious ZIP paths, oversized archives, digest mismatches, unsupported model content, and attempts to smuggle executable files.

## Implementation plan

### M0 — Do not implement MCP yet

Finish the component schemas, registry service, additive `SceneEntity` migration, and generic primitive renderer first.

Exit criterion:

- the application can create/validate/use components without MCP.

### M1 — Integrated read-only MCP

Deliver:

```text
/mcp in Go API
components.list
components.get
components.validate
farms.get_layout
schema resources
```

Exit criterion:

- an MCP client can inspect the real registry/layout without repository access.

### M2 — Primitive component authoring

Deliver:

```text
components.create
```

Exit criterion:

- an agent can create an immutable valid primitive component revision and retrieve the same revision by exact digest.

### M3 — Farm binding mutation

Deliver:

```text
farms.set_component
farms.validate_layout
```

Use the existing farm version for compare-and-swap.

Exit criterion:

- an agent can migrate/build the reference pilot component bindings without creating duplicate domain identities.

### M4 — Pack installation/export

Only after browser/server registry persistence and archive v2 exist.

### M5 — GLB/draft authoring

Only after model validation/storage exists. Use explicit GrowNerve draft IDs, never implicit MCP session state.

### M6 — Physical command tools

Optional and separate. Only add them if there is a concrete operator use case; they must call the existing command/safety path.

## Acceptance criteria

The MCP integration is successful when:

- no tool generates or edits Three.js source code.
- tool and UI paths use the same component validator.
- scene writes preserve existing `(entity_type, entity_id)` identity.
- exact component revision/digest is persisted.
- retries cannot silently create duplicate component revisions or scene entities.
- farm writes obey existing compare-and-swap semantics.
- current role authorization applies to every call.
- no component metadata can grant additional privileges.
- no pack can install executable code in V0.
- browser-only archives remain usable without MCP.
- an MCP client can create a primitive component and bind it to the pilot farm end-to-end.

## First proof to build

Do not start with the large future tool catalog.

The highest-value proof is:

```text
components.list
components.get
components.validate
components.create
farms.get_layout
farms.set_component
farms.validate_layout
```

That is enough to prove the core idea: an AI can add a new validated component to GrowNerve while the existing domain model, concurrency boundary, permissions, browser portability, and renderer architecture remain intact.