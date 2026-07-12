# Beta scope

COE public beta supports narrow self-host path.

## Supported

- self-hosted `ka`
- self-hosted `coe-signal`
- optional TURN for internet users
- Windows desktop agent via `coe-node`
- voucher-based enroll
- TCP direct peer path
- WebRTC peer path
- one operator / one org style deployments

## Supported deployment shapes

| Shape | Status |
|------|--------|
| single-host lab | supported |
| one KA + one signal + one TURN | supported |
| reverse proxy TLS in front | supported |

## Not supported in beta

- hosted COE service by project
- mobile clients
- Kubernetes / HA KA
- multi-region failover
- anonymous routing / metadata privacy
- post-quantum crypto suite
- custom auth plugins
- LocalSystem Windows service for user DPAPI identities

## Support boundary

If operator changes infra outside docs, operator owns debugging first.

Expected bug reports include:

- agent version
- OS
- `ka-check` output
- signal / TURN config shape
- sanitized logs
