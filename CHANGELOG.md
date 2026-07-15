# Changelog

## Unreleased (post v0.1.0-beta)

### Phase 1 — beta hardening
- Agent clock offset from KA `health` / `ka_time` (reduce issue clock_skew)
- `ka-check -stun` / `-turn` UDP path probes
- `ka -audit-file` JSONL audit export (stdout + file)

### Phase 2 — operator infrastructure
- SQLite registry: `ka -sqlite path.db` (JSON file still default)
- Admin UI at `/admin` (vouchers, devices, revoke)
- `GET /v1/devices` admin list

## Unreleased (post v0.1.0-beta)

### Phase 1 — beta hardening
- Agent clock offset from KA `health` / `ka_time` (reduce issue clock_skew)
- `ka-check -stun` / `-turn` UDP path probes
- `ka -audit-file` JSONL audit export (stdout + file)

### Phase 2 — operator infrastructure
- SQLite registry: `ka -sqlite path.db` (JSON file still default)
- Admin UI at `/admin` (vouchers, devices, revoke)
- `GET /v1/devices` admin list

## v0.1.0-beta

- COE protocol spec published
- desktop Windows agent with DPAPI key protection
- self-host Key Authority and signaling server
- voucher-based enroll flow
- WebRTC peer path with STUN/TURN config hooks
- Docker self-host files
- Bare-metal (no Docker) self-host guide
- `ka-check` production validation tool
- GitHub release and release assets
- Public beta announcement (`ANNOUNCEMENT.md`)

## Compatibility note

Beta release. Breaking changes may happen between beta tags.
