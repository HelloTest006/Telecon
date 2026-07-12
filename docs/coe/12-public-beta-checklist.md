# Public beta checklist

Status at upload time.

## Complete in repo

- [x] Protocol spec published under `docs/coe/`
- [x] Desktop agent (`coe-node`) with DPAPI-backed identity/key storage on Windows
- [x] TLS to Key Authority; signed daily key issue
- [x] Voucher-based enroll flow (`coe-admin voucher`, `coe-node -enroll -voucher ...`)
- [x] WebRTC path with signaling server (`coe-signal`) and TURN/STUN config hooks
- [x] Self-host docs + Docker (`Dockerfile`, `docker-compose.yml`)
- [x] `ka-check` automated production checks
- [x] CI / release workflow files under `.github/workflows/`
- [x] License split documented (`LICENSE`, `LICENSES/`, `NOTICE`)

## Verified locally before upload

- [x] `go test ./...`
- [x] `scripts/smoke-voucher.ps1`
- [x] `scripts/smoke-agent.ps1`
- [x] `scripts/smoke-webrtc.ps1`

## Manual ops gates before broad public beta

- [ ] Hosted KA on real domain with non-default admin token
- [ ] Public CA or org CA for KA and signal TLS
- [ ] Backup/restore drill for `KA_MASTER` and registry
- [ ] TURN reachable from target networks
- [ ] Signed Windows binaries / installer package
- [ ] Terms, privacy, support contact, issue template
- [ ] Beta announcement copy uses honest security claims

## Recommended first public-beta runbook

1. Deploy `ka` and `coe-signal` with TLS.
2. Run `ka-check` against deployment.
3. Mint one-time vouchers for testers.
4. Ship packaged Windows agent build.
5. Collect failures from install, enroll, WebRTC, and revoke flows.
