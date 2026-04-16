# Anti-Spam Registration (Tier 2, No CAPTCHA)

## Overview

This task upgrades the current registration anti-spam system (assumes honeypot + timing) to a **Tier 2 solution** that significantly improves resistance to automated abuse while preserving a frictionless user experience.

**Constraints:**

* No CAPTCHA or third-party dependencies
* Must remain lightweight and Go-native
* Must integrate cleanly with existing HTTP handlers and Postgres

---

## Goals

* Prevent automated registrations from simple and intermediate bots
* Avoid user friction (no puzzles, no external services)
* Maintain deterministic, testable server-side logic
* Minimize operational complexity

---

## Non-Goals

* This is not intended to stop advanced, targeted attacks using headless browsers
* This does not replace full WAF or enterprise bot mitigation systems

---

## Design Summary

The system combines:

1. **Signed form token (HMAC)**
2. **Minimum and maximum submit timing checks**
3. **Dynamic honeypot field**
4. **Optional single-use nonce**
5. **Basic rate limiting**
6. **Uniform error responses**

---

## Architecture

### Form Lifecycle

Note: Assumes `/register` is the self-registration route.

#### GET `/register`

* Generate token payload
* Sign token
* Generate dynamic honeypot field name
* Render form with hidden fields

#### POST `/register`

* Validate token signature
* Validate token expiration
* Validate submission timing
* Validate honeypot field is empty
* Validate nonce (if enabled)
* Apply rate limiting
* Proceed or reject

---

## Token Specification

### Structure

```
FormToken {
    Form      string   // "register"
    IssuedAt  int64    // unix timestamp
    ExpiresAt int64    // unix timestamp
    Honeypot  string   // randomized field name
    Nonce     string   // optional unique ID
}
```

### Encoding

* Serialize (JSON or compact format)
* Sign using HMAC (SHA-256 recommended)
* Return as base64 or hex string

Example:

```
base64(payload) + "." + base64(signature)
```

---

## Validation Rules

### 1. Signature Validation

* Reject if HMAC signature is invalid

### 2. Expiration Check

* Reject if `now > ExpiresAt`

### 3. Minimum Time Check

* Reject if `now - IssuedAt < 2 seconds`

### 4. Maximum Time Check

* Reject if `now - IssedAt > 30–60 minutes`

### 5. Honeypot Check

* Reject if honeypot field is non-empty

### 6. Nonce Check (Optional)

* Reject if nonce already used
* Mark nonce as used after validation

### 7. Rate Limiting

* Example policy:

  * Max 5 attempts per IP per hour
  * Apply stricter limits on repeated failures

---

## Honeypot Design

### Requirements

* Field must:

  * Be invisible to users
  * Be present in DOM
  * Use randomized name per request

### Example Rendering

```html
<input type="text" name="hp_8f3a2c" style="position:absolute; left:-9999px;">
```

### Notes

* Do NOT use obvious names like `honeypot`
* Avoid `display:none` if possible (bots may ignore)
* Ensure accessibility is not negatively impacted

---

## Rate Limiting

### Minimum Implementation

* Key: IP address
* Storage:

  * In-memory map OR SQLite table

### Example Table (SQLite)

```sql
CREATE TABLE rate_limits (
    ip TEXT,
    attempts INTEGER,
    last_attempt INTEGER
);
```

---

## Error Handling

### Rules

* All failures return the same response
* Do NOT reveal which validation failed

### Example Response

```json
{
  "error": "Invalid submission"
}
```

Optional:

* Return HTTP 200 with generic success message for silent rejection

---

## Logging

Log the following fields for analysis:

* IP address
* User agent
* Token age
* Honeypot triggered (bool)
* Rate limit triggered (bool)

---

## Example Server Flow (Go)

```go
func handleRegister(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()

    token := r.FormValue("form_token")

    data, err := verifyToken(token)
    if err != nil {
        reject(w)
        return
    }

    now := time.Now().Unix()

    if now < data.IssuedAt+2 {
        reject(w)
        return
    }

    if now > data.ExpiresAt {
        reject(w)
        return
    }

    if r.FormValue(data.Honeypot) != "" {
        reject(w)
        return
    }

    if isRateLimited(r.RemoteAddr) {
        reject(w)
        return
    }

    if nonceUsed(data.Nonce) {
        reject(w)
        return
    }

    markNonceUsed(data.Nonce)

    // proceed with registration
}
```

---

## Security Notes

* Never reseed token signing key at runtime
* Store signing secret securely (env/config)
* Tokens must be tamper-proof (HMAC required)
* Do not trust any client-provided values without verification

---

## Testing Strategy

### Unit Tests

* Valid token passes
* Expired token fails
* Tampered token fails
* Honeypot filled fails
* Too-fast submission fails
* Too-old submission fails

### Integration Tests

* Full GET → POST flow
* Replay attack attempt
* Rate limit enforcement

---

## Implementation Checklist

* [ ] Token generation (GET handler)
* [ ] Token verification (POST handler)
* [ ] HMAC signing utility
* [ ] Dynamic honeypot field generation
* [ ] Timing validation
* [ ] Rate limiter
* [ ] Optional nonce store
* [ ] Logging hooks
* [ ] Uniform error responses
* [ ] Tests (unit + integration)

---

## Acceptance Criteria

* Registration form works normally for human users
* Automated submissions without valid tokens are rejected
* Honeypot catches naive bots
* Replay and stale submissions are rejected
* No visible UX impact

---

## Future Enhancements (Optional)

* Progressive challenges after repeated failures
* IP reputation scoring
* Behavioral fingerprinting
* Optional CAPTCHA fallback for flagged users

---

## Summary

This approach provides a **low-friction, high-signal anti-spam layer** suitable for small to medium applications:

* No external dependencies
* Stronger than honeypot alone
* Easy to implement in Go
* Compatible with SQLite and stateless APIs
