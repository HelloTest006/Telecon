# COE Key Authority API

## Transport

| Item | Requirement |
|------|-------------|
| Protocol | HTTPS only |
| TLS | 1.3 preferred; 1.2 minimum if unavoidable |
| Base path | `/v1` |
| Body encoding | CBOR (`Content-Type: application/cbor`) preferred; JSON (`application/json`) allowed for debug |
| Time | All timestamps Unix UTC seconds (int64) |

## Authentication

### Device credentials (v1)

| Method | Description |
|--------|-------------|
| **mTLS** (recommended) | Device client cert issued at enrollment; KA maps cert fingerprint → `device_id` |
| **Bearer enrollment token** | Long-lived token only for bootstrap; after enroll, prefer mTLS |
| **Signed challenge** | KA returns `challenge`; device signs with identity key; optional extra binding |

Every key-issue request **MUST** authenticate the device. Unauthenticated requests → `401`.

### KA identity (device side)

Devices **MUST** validate KA server certificate (or pin SPKI/public key). Do not disable TLS verification in production profiles.

## Endpoints

### `GET /v1/health`

Unauthenticated liveness.

**Response 200**

```json
{
  "status": "ok",
  "time": 1783814400,
  "epoch_id": 20646
}
```

---

### `POST /v1/enroll`

Admin or bootstrap enrollment. **Not** called daily by devices in steady state.

**Auth:** admin credential or one-time enrollment voucher.

**Request**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `device_id` | bstr / string | yes | Unique device id |
| `identity_pk` | bstr (32) | yes | X25519 public key (COE-Strong) |
| `org_id` | string | no | Org for COE-Simple / policy |
| `label` | string | no | Human label |
| `enrollment_counter` | uint | no | Default 1; bump on re-enroll |

**Response 201**

| Field | Type | Description |
|-------|------|-------------|
| `device_id` | | Echo |
| `enrollment_counter` | uint | Stored counter |
| `client_cert` | bstr / PEM | If KA mints mTLS cert |
| `profile` | string | `"strong"` or `"simple"` |

**Errors:** `409` duplicate device; `403` voucher invalid.

---

### `POST /v1/key/issue`

**Primary daily call.** Device obtains general key for current epoch.

**Auth:** device mTLS or equivalent.

**Request**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `device_id` | bstr / string | yes | Must match auth binding |
| `epoch_hint` | uint | no | Client’s idea of epoch; KA may ignore if skew ok |
| `nonce` | bstr (16) | yes | Client random; anti-replay in audit |
| `client_time` | int64 | yes | Device UTC seconds |
| `profile` | string | no | `"strong"` (default) or `"simple"` |

**Processing**

1. Authenticate device; reject if revoked.
2. Compute `epoch_id` from KA clock (authoritative).
3. If `|client_time - ka_time| > 120` → `400 clock_skew` (unless policy relaxes).
4. If `epoch_hint` present and differs from KA epoch by more than 1 → `400 epoch_mismatch`.
5. Derive `GeneralKey` per [02-key-management](02-key-management.md).
6. Advance Xoroshiro-128++ for `request_id` / public tags.
7. Persist audit row: `device_id`, `epoch_id`, `request_id`, `key_serial`, time, IP — **not** raw key.
8. Return key material.

**Response 200**

| Field | Type | Description |
|-------|------|-------------|
| `device_id` | | Echo |
| `epoch_id` | uint | Authoritative epoch |
| `not_before` | int64 | |
| `not_after` | int64 | |
| `grace_seconds` | uint | Default `300` |
| `key_serial` | uint | Monotonic per device or global |
| `request_id` | bstr / uint | Xoroshiro-backed public ticket |
| `public_tag` | bstr | Optional non-secret mix tag |
| `general_key` | bstr (32) | Secret daily key |
| `profile` | string | `"strong"` or `"simple"` |
| `ka_time` | int64 | For client clock adjust |

**Idempotency:** Same device + same `epoch_id` → same `general_key` and same `key_serial`. New `request_id` each HTTP call is allowed for audit.

**Errors**

| Code | `error` | When |
|------|---------|------|
| 401 | `unauthorized` | Bad/missing auth |
| 403 | `revoked` | Device revoked |
| 403 | `not_enrolled` | Unknown device |
| 400 | `clock_skew` | Time out of range |
| 400 | `epoch_mismatch` | Hint too far off |
| 400 | `bad_request` | Schema |
| 429 | `rate_limited` | Abuse (still may return same key if already issued this epoch — implementer choice; prefer 200 idempotent over 429 for honest retries) |
| 500 | `internal` | No key bytes in body |

---

### `GET /v1/key/status`

**Auth:** device.

**Query:** none (device from auth) or `?device_id=` for admin.

**Response 200**

| Field | Type | Description |
|-------|------|-------------|
| `device_id` | | |
| `revoked` | bool | |
| `current_epoch_id` | uint | |
| `issued_this_epoch` | bool | Whether key was issued |
| `key_serial` | uint / null | If issued |
| `not_after` | int64 / null | |

No `general_key` in status response.

---

### `POST /v1/revoke` (admin)

**Auth:** admin.

**Request:** `{ "device_id": ..., "reason": "..." }`

**Response 200:** `{ "device_id", "revoked": true }`

Subsequent `/v1/key/issue` → `403 revoked`.

---

## Rate limits

| Scope | Recommendation |
|-------|----------------|
| Per device issue | Soft: 60/hour; hard burst 10/min |
| Same epoch re-issue | Always idempotent 200 with same key |
| Global unauth | Strict on `/v1/enroll` |

## Audit log fields (normative minimum)

```
timestamp, request_id, device_id, epoch_id, key_serial,
endpoint, src_ip, result_code, profile
```

**Forbidden in logs:** `general_key`, `KA_MASTER`, TLS session tickets that embed secrets.

## Example (JSON illustration)

**Request `POST /v1/key/issue`**

```json
{
  "device_id": "dev-alpha-001",
  "epoch_hint": 20646,
  "nonce": "base64-16-bytes",
  "client_time": 1783814500,
  "profile": "strong"
}
```

**Response**

```json
{
  "device_id": "dev-alpha-001",
  "epoch_id": 20646,
  "not_before": 1783814400,
  "not_after": 1783900800,
  "grace_seconds": 300,
  "key_serial": 42,
  "request_id": "…",
  "public_tag": "…",
  "general_key": "…32 bytes…",
  "profile": "strong",
  "ka_time": 1783814501
}
```

## Security notes

- Prefer short-lived TLS sessions; no caching of response bodies in shared proxies.
- `Cache-Control: no-store` on all key responses.
- Separate admin listener or strict RBAC for enroll/revoke.
