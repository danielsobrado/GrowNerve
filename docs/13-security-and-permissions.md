# 13 — Security and Permissions

## Security goals

GrowNerve is a control system as well as a farm record. Unauthorized reads are undesirable; unauthorized actuator commands can damage crops or equipment. Authorization therefore applies to both data and control actions.

## Authentication

V0 local deployments may support a simple local authentication mode for development, but production-capable architecture should support OIDC/OAuth2 without coupling domain logic to an identity provider.

Recommended modes:

```text
dev      local development only
local    small local deployment
OIDC     production / managed deployment
```

Production configuration must reject development bypass modes.

## Roles

Initial roles can remain small:

```text
viewer
operator
manager
administrator
```

Typical permissions:

### Viewer

- read current farm state
- view history
- view 3D twin

### Operator

- viewer rights
- add observations
- acknowledge alerts
- issue permitted low-risk commands
- record inputs/harvest

### Manager

- operator rights
- edit recipes/configuration
- manage automation rules
- approve higher-risk actions where configured

### Administrator

- users/roles
- device provisioning
- system configuration

## Command authorization

Authorization and safety are separate checks.

A user may be authorized to request a dosing command but the command can still be rejected by safety interlocks.

```text
authorized? -> safety-valid? -> accepted
```

Neither check may be implemented only in the browser.

## 3D permissions

Radial menus can hide/disable actions based on permissions, but this is convenience only. The API revalidates permissions for every write/command.

## Device authentication

Prefer unique credentials per ESP32/device.

Requirements:

- device identity is explicit
- credential rotation is possible
- disabled/decommissioned devices cannot publish trusted telemetry
- broker ACLs restrict topics to device scope where practical

## Secrets

Secrets never live in committed YAML files.

Use environment variables or mounted secret files for:

```text
database credentials
OIDC client secrets
MQTT credentials
object-storage credentials
signing keys
```

Provide `.env.example` with names, never real values.

## Network exposure

Default local deployment should bind control services conservatively. Do not expose PostgreSQL or MQTT to public networks by default.

Remote access should preferably use VPN/reverse proxy with TLS rather than exposing edge devices directly to the Internet.

## Audit

Security-relevant actions must record:

- actor
- action
- target
- timestamp
- correlation ID
- command/result where applicable

High-value audit examples:

```text
login failure
role change
device credential lifecycle
automation enable/disable
manual override
emergency stop clear
hazardous command request/rejection
```

## API protections

- request body limits
- rate limiting where exposed beyond trusted LAN
- strict content types
- CORS allow-list
- security headers
- panic recovery
- structured safe errors
- input validation
- no raw SQL errors to clients

## Media

Uploaded images should be validated by content type/size and stored under generated IDs rather than user-controlled filesystem paths.

## Threat assumptions

V0 should explicitly defend against:

- accidental command duplication
- stale browser state
- compromised/unknown device publishing telemetry
- malformed MQTT payloads
- unauthorized control requests
- replayed device commands
- exposed default credentials

Advanced enterprise threat models can be added later without weakening these basics.
