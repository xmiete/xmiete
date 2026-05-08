# XMiete Core API Specification

This document describes the RESTful API for managing digital rental deposits according to the XMiete standard.

## Base URL
`https://api.xmiete.org/v1`

## Authentication & Security
All requests must be authenticated using **OAuth2 Bearer Tokens**.
`Authorization: Bearer <token>`

For detailed security requirements, including **mTLS** and **JWS signing** of critical state changes, refer to [SECURITY.md](./SECURITY.md).

### Required Scopes
- `POST /deposits`: `deposit:create`
- `GET /deposits/{id}`: `deposit:read`
- `PATCH /deposits/{id}/identity`: `deposit:write`
- `POST /deposits/{id}/pledge`: `deposit:pledge`
- `POST /deposits/{id}/release`: `deposit:release`
- `POST /deposits/{id}/claim`: `deposit:claim`

## Core Endpoints

### 1. Application Management

#### `POST /deposits`
Creates a new rental deposit request.
- **Initial State:** `REQUESTED`
- **Payload:** Full `xmiete_schema.json` object (excluding `history` and `pledge` confirmation).

#### `GET /deposits/{id}`
Retrieves the current state and full history of a deposit.

### 2. Identity & Verification

#### `PATCH /deposits/{id}/identity`
Updates the eID/KYC status.
- **Trigger:** Moves state to `IDENTIFIED` if verification is successful.
- **Payload:**
  ```json
  {
    "eid_status": "VERIFIED",
    "verification_timestamp": "2026-05-07T10:00:00Z",
    "provider_reference": "EID-ABC-123"
  }
  ```

### 3. Funding & Pledging

#### `POST /deposits/{id}/pledge`
The bank/insurer confirms the legal pledge (BGB § 551).
- **Trigger:** Moves state to `PLEDGED`.
- **Payload:**
  ```json
  {
    "pledge_date": "2026-05-07",
    "is_confirmed_by_bank": true,
    "provider_reference": "PLEDGE-XYZ-789"
  }
  ```

### 4. Tenancy Management (Release & Claim)

#### `POST /deposits/{id}/release`
The landlord authorizes the release of the deposit.
- **Trigger:** Moves state to `RELEASED`.
- **Payload:**
  ```json
  {
    "release_type": "FULL",
    "release_amount": 1500.00,
    "landlord_signature_token": "SIGN-998877"
  }
  ```

#### `POST /deposits/{id}/claim`
The landlord initiates a claim against the deposit.
- **Trigger:** Moves state to `CLAIMED`.
- **Payload:**
  ```json
  {
    "claim_amount": 450.00,
    "reason": "Damages to flooring",
    "evidence_urls": ["https://.../photo1.jpg"]
  }
  ```

## Webhooks
Stakeholders should implement a webhook listener to receive asynchronous status updates.

**Event Payload:**
```json
{
  "event_type": "deposit.status_changed",
  "deposit_id": "UUID-123-456",
  "new_state": "PLEDGED",
  "timestamp": "2026-05-07T10:05:00Z"
}
```

## Error Handling
XMiete APIs use standard HTTP status codes:
- `400 Bad Request`: Validation failed (e.g., malformed JSON or illegal state transition).
- `401 Unauthorized`: Invalid token.
- `403 Forbidden`: Insufficient permissions for this specific deposit.
- `404 Not Found`: Deposit ID does not exist.
- `409 Conflict`: Transition not allowed from the current state.

---
**License:** Licensed under [Creative Commons Attribution 4.0 International (CC BY 4.0)](./LICENSE-SPECIFICATION).
