# COE public beta

**Self-hosted only.** This project does **not** run auth, key, signaling, or message infrastructure for you.

COE (Communication over Encryption) is open-source software for operators who want **device-to-device** encrypted messaging under **their own** Key Authority. Each operator deploys and operates:

- **Key Authority (`ka`)** — daily key issue, enroll vouchers, revoke
- **Signaling (`coe-signal`)** — WebRTC SDP/ICE only (not message content)
- **Optional TURN** — for hard NAT / internet peers
- **Desktop agent (`coe-node`)** — on each device

If you cannot host those pieces, COE is not the right product yet.

## Honest security claims

- Messages do not transit the Key Authority.
- Daily keys rotate on a 24h epoch.
- Confidentiality requires secrecy of daily keys and long-term device identity material.
- **Not** “uncrackable.” Beta software. Use at your own risk.

Full threat model: [docs/coe/01-threat-model.md](docs/coe/01-threat-model.md).

## Who this beta is for

- Operators who will run their own stack
- Small teams / labs testing peer-to-peer encrypted transport
- People who want an open protocol + self-host AGPL servers + Apache clients

## Who this beta is not for

- Users expecting a hosted chat service
- Production critical use without own review and ops
- Mobile-first deployments (not in beta)

## How to try it

1. Read [docs/self-host-quickstart.md](docs/self-host-quickstart.md)
2. Deploy `ka` + `coe-signal` (Docker or binaries)
3. Run `ka-check` against your deployment
4. Mint a voucher with `coe-admin`
5. Install Windows agent with `scripts/install-agent.ps1`
6. Enroll with voucher; connect peers (TCP or WebRTC)

Scope and support boundary: [BETA_SCOPE.md](BETA_SCOPE.md)  
Self-host security: [SECURITY_SELFHOST.md](SECURITY_SELFHOST.md)

## Licenses

| Part | License |
|------|---------|
| Protocol spec | Apache-2.0 |
| Client / agent / libraries | Apache-2.0 |
| Key Authority, signal, admin tools, server images | AGPL-3.0-or-later |

See [LICENSE](LICENSE) and [LICENSES/](LICENSES/).

## Feedback

- Bugs / beta feedback: GitHub Issues (templates included)
- Do not paste secrets, admin tokens, vouchers, or raw keys into issues

---

# Release notes — v0.1.0-beta

**Tag:** `v0.1.0-beta`  
**Release:** https://github.com/HelloTest006/Telecon/releases/tag/v0.1.0-beta  
**Repo:** https://github.com/HelloTest006/Telecon

## Highlights

- Open COE protocol specification under `docs/coe/`
- Self-hosted Key Authority with TLS, signed daily key issue, rate limits, `-prod` mode
- Voucher-based enroll (devices never need admin token)
- Windows desktop agent with DPAPI-protected identity and general keys
- User logon agent install (Task Scheduler; DPAPI-safe)
- P2P over TCP and WebRTC data channels (KEY_OFFER exchange; no KA peer-key dump)
- STUN/TURN configuration hooks for agents
- `coe-signal` for SDP/ICE only
- `ka-check` deployment checker
- Docker images / compose for self-host
- CI and release workflows

## Binaries (GitHub release assets)

Verify with `SHA256SUMS` on the release page.

| Asset | Role |
|-------|------|
| `ka-linux-amd64` | Key Authority (AGPL) |
| `coe-signal-linux-amd64` | Signaling (AGPL) |
| `coe-admin-linux-amd64` | Admin CLI (AGPL) |
| `ka-check-linux-amd64` / `ka-check-windows-amd64.exe` | Deploy checks |
| `coe-node-linux-amd64` / `coe-node-windows-amd64.exe` | Device agent (Apache) |
| `coe-cli-linux-amd64` / `coe-cli-windows-amd64.exe` | Local API client |
| `coe-keygen-linux-amd64` / `coe-keygen-windows-amd64.exe` | Identity generation |
| `SHA256SUMS` | Checksums |

Windows binaries in this beta are **unsigned** (SmartScreen may warn). Prefer hashes from `SHA256SUMS`. Code signing is a later milestone.

Local zip from `scripts/build-release.ps1` (optional packaging):

```text
coe-0.1.0-beta.zip
SHA256: 45D1CAD48EF6E809182C5AEC74A5E0FD12CFF3012FD86774E99B017E0E12EB1B
```

(GitHub Actions multi-file assets are the canonical download for this tag.)

## Supported topologies

- Single-host lab
- One KA + one signal + one TURN
- TLS reverse proxy in front (Caddy example in repo)

## Known limits

- No project-hosted KA / signal / TURN
- No mobile clients
- No HA / multi-region KA
- No metadata privacy / anonymity network
- Direct TCP needs reachability; WebRTC often needs TURN off-LAN
- Beta may break between tags — see [COMPATIBILITY.md](COMPATIBILITY.md)

## Operator checklist (minimum)

1. Strong `COE_KA_ADMIN_TOKEN` (never default)
2. TLS on KA (no plain HTTP in prod)
3. Backup `master.key` + `registry.json`
4. `ka-check` exit 0
5. Vouchers for enroll; revoke runbook ready
6. TURN if peers are not on same LAN

## Crypto summary (v1 suite)

- Daily material: HKDF-SHA-256 from server master + epoch + device
- Session: X25519 (static + ephemeral) + both daily keys (Strong profile)
- Records: ChaCha20-Poly1305
- Xoroshiro-128++: non-secret tickets only

## Upgrade

Beta family only: match agent and KA to same beta line. No long-term stability promise yet.

## Links

- Quickstart: [docs/self-host-quickstart.md](docs/self-host-quickstart.md)
- Spec index: [docs/coe/README.md](docs/coe/README.md)
- Changelog: [CHANGELOG.md](CHANGELOG.md)
- Privacy / Terms: [PRIVACY.md](PRIVACY.md), [TERMS.md](TERMS.md)
