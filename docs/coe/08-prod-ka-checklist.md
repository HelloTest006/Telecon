# Production Key Authority checklist

Run on the KA host and from a client:

```text
ka-check -url https://ka.example.com -ca /path/to/ca-or-server.crt -admin-token $TOKEN
ka-check -url https://ka.example.com -ca ... -master-file /var/lib/coe/master.key -registry /var/lib/coe/registry.json
```

Exit code **0** = all **required** checks passed.

## Required (automated)

| ID | Meaning |
|----|---------|
| `tls_url` | Base URL is `https://` |
| `tls_verify` | Not using `-insecure` |
| `health` | `GET /v1/health` returns ok |
| `tls_handshake` / `tls_version` | TLS 1.2+ |
| `cert_expiry` | ≥14 days validity |
| `clock_skew` | \|client − KA\| ≤ 120s |
| `admin_token_set` | Not default `dev-admin-token` |
| `master_file` | 32-byte master when path given |

## Required (manual / ops)

- [ ] `KA_MASTER` in HSM or encrypted volume; **never** in git
- [ ] Registry + master **backed up** offline; restore drill done
- [ ] Admin token rotated; stored in secret manager
- [ ] Firewall: only 443 (or chosen TLS port) public; admin enroll restricted
- [ ] Audit logs to append-only store; **no** `general_key` in logs
- [ ] Device enroll process documented (voucher / MDM)
- [ ] Revoke runbook for lost devices
- [ ] NTP on KA and devices
- [ ] Public or org CA (not long-lived self-signed) for production clients

## Warnings

| ID | Meaning |
|----|---------|
| `cert_public` | Self-signed OK for lab only |
| `ca_file` | System roots only — pin CA in agents |

## Agent side

- Use **logon agent** (`install-agent.ps1`), not LocalSystem service, for DPAPI.
- Ship same CA PEM as `-ka-ca` / `ka_ca_file` in node config.
- `ka_insecure: false` always in production configs.
