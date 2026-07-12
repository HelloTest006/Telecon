# Enroll vouchers

Devices should **not** receive the long-lived admin token. Admins mint **vouchers**.

## Flow

```
Admin ──POST /v1/vouchers──► KA  (Bearer admin)
Admin ──code (once)────────► User
User  ──POST /v1/enroll────► KA  { voucher_code, keys... }
KA    ──redeems hash, enrolls device──►
User  ──POST /v1/key/issue─► KA  (Ed25519 signed, no voucher)
```

## API

### `POST /v1/vouchers` (admin)

```json
{"max_uses":1,"ttl_hours":168,"label":"alice","org_id":"","profile":"strong"}
```

Response includes plaintext `code` **once**. KA stores only `sha256(code)`.

### `POST /v1/enroll` (device)

```json
{
  "device_id":"alice",
  "identity_pk":"...",
  "sign_pk":"...",
  "voucher_code":"..."
}
```

No `Authorization` header required when voucher present.

### Errors

`invalid_voucher`, `voucher_expired`, `voucher_exhausted`, `voucher_revoked`

## CLI

```text
coe-admin voucher -max-uses 1 -ttl-hours 72
coe-node -enroll -voucher <code> ...
```
