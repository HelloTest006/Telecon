# Changelog

## Unreleased (post v0.1.0-beta)

### Phase 1 — beta hardening
- Agent clock offset from KA `health` / `ka_time` (reduce issue clock_skew)
- `ka-check -stun` / `-turn` UDP path probes
- `ka -audit-file` JSONL audit export (stdout + file)

### Phase 2 — operator infrastructure
- SQLite registry: `ka -sqlite path.db` (JSON file still default)
- PostgreSQL registry: `ka -postgres 'postgres://...'`
- Admin UI at `/admin` (vouchers, devices, revoke)
- `GET /v1/devices` admin list
- Agent update check: `GET /v1/update/check` + `ka -update-manifest file.json` (unsigned beta; agent logs only)

### Phase 3 — client UX and transport
- `coe-tray` local desktop UI (opens browser; proxies to coe-node API)
- WebRTC ICE: multi-STUN defaults, `COE_ICE_POLICY=all|relay`, env fill-in
- Linux: `scripts/install-agent-linux.sh` (systemd --user)
- macOS: `scripts/install-agent-macos.sh` (LaunchAgent)

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
