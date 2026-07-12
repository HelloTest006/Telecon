# Self-host security notes

COE is self-hosted. Operator owns backend security.

## Must do

- use strong `COE_KA_ADMIN_TOKEN`
- use TLS for `ka`
- use TLS for signal or put signal behind TLS reverse proxy
- store `KA_MASTER` outside git
- back up `KA_MASTER` and registry
- restrict local agent API to loopback only
- rotate voucher / admin secrets on leak

## Must not do

- do not run production with `-http`
- do not keep `dev-admin-token`
- do not expose `http://127.0.0.1:7701` to internet
- do not send raw `general_key` in logs or tickets
- do not run user DPAPI keys under LocalSystem service

## Incident notes

### Admin token leak

1. rotate `COE_KA_ADMIN_TOKEN`
2. revoke suspicious devices
3. reissue vouchers
4. inspect audit logs

### `KA_MASTER` leak

Treat as severe. Rotate deployment, re-enroll devices, invalidate trust path.

### Lost device

1. revoke device
2. mint new voucher
3. re-enroll replacement device

## Network

Public beta typical ports:

- `443` TLS reverse proxy
- `3478` TURN
- `5349` TURN TLS
- TURN relay UDP range (`49152-65535` in current example)

## Logging

Keep:

- enroll events
- issue events
- revoke events
- signal operational logs

Do not keep:

- `general_key`
- private keys
- voucher plaintext after display
