# Authentication System Design — Synkademy
___

## Purpose

This document specifies the production authentication and authorization architecture for Synkademy. It supersedes the original v1 plan, resolving critical security gaps and completing underspecified decisions.

The design provides secure identity management today while preserving a clear, low-friction migration path to RBAC (host / instructor / student / admin) without rewriting the auth core.

## Goals

- Secure login, logout, and session renewal for browser clients.
- Short-lived access tokens with rotating, revocable refresh sessions.
- Zero credential or long-lived secret exposure to the browser.
- An authorization layer that is trivial today and extensible to full RBAC.
- Go backend and React frontend deployable together or separately.

## Non-Goals

- Social login or SSO providers.
- Multi-tenant organization management.
- Fine-grained RBAC rule enforcement.

___

## Recommended Approach

Use a hybrid token model:

- Short-lived access token as a JWT (RS256).
- Rotating refresh token stored as an `HttpOnly`, `Secure`, `SameSite=Strict cookie.
- Server-side session store for refresh-token tracking, revocation, rotation and device
  visibilty.

  The access token keeps API requests fast and stateless. The refresh session provides revocation, device management, and reuse detection. Keeping the refresh token in an HttpOnly cookie eliminates the XSS exposure that localStorage creates. RS256 signing allows future decoupled resource servers to verify tokens without sharing a symmetric secret.

___

## Core Principles

1. Authentication proves identity. Authorization decides what that identity may do.
2. Access tokens are short-lived, stateless, and disposable.
3. Refresh tokens are opaque, rotated on every use, and revocable centrally.
4. The database is the source of truth for user status, roles, and permission grants.
5. The frontend never touches passwords or long-lived secrets.
6. All sensitive comparisons use constant-time functions.

___

## Identity Model
One user type in v1: `user`. All authenticated users have equal rights. The role field is stored now so future RBAC requires no schema migration.

### User Table

    ```
    id UUID PRIMARY-KEY GENERATEDVALUE
    email TEXT UNIQUE LOWERCASE INDEXED
    password_hash TEXT
    display_name TEXT
    status ENUM(active, disabled, pending_verification, deleted)
    role TEXT DEFAULT(user)
    email_verified_at TIMESTAMPZ
    created_at TIMESTAMPZ
    updated_at TIMESTAMPZ
    last_login_at TIMESTAMPZ
    ```

### Session Table
    ```
    id UUID PRIMARY-KEY GENERATEDVALUE
    family_id UUID # Shared across all rotations of the same session chain
    user_id UUID FOREIGNKEY
    refresh_token_hash TEXT # SHA256
    status ENUM(active, rotated, revoked)
    expires_at TIMESTAMPZ
    revoked_at TIMESTAMPZ
    rotated_from_id UUID # Parent session id, for rotation chain auditing
    ip_addres INET
    user_agent TEXT
    created_at TIMESTAMPZ
    last_seen_at TIMESTAMPZ # Updated on each successful refresh
    ```

The family_id field groups all rotation-linked sessions together. When reuse of a rotated token is detected, the entire family is revoked in one query — no need to walk the chain.

___

## Password Policy
### Use Argon2id for all password hashing.

- Minimum length: 12 characters.
- Maximum length: 128 characters (prevent denial-of-service via bcrypt-style length extension on future algorithm changes).
- Recommended Argon2id parameters (tune to ~200–300ms on production hardware): memory=64MB, iterations=3, parallelism=4.
- Never store plaintext or reversibly encrypted passwords.
- Return a constant-time generic error for both wrong email and wrong password (see Timing Side-Channels below).
- Consider optional pwned-password breach checking via the k-anonymity Have I Been Pwned API in a future phase.

___

## Token Strategy
### Access Token (JWT)

- Format: JWT, signed with RS256 (asymmetric).
- Lifetime: 15 minutes.
- Transport: Authorization: Bearer <token> header for API calls.
- Frontend storage: memory only — never localStorage, never sessionStorage.

### Required claims:
```
  {
    "sub": "user Id",
    "email: "user email",
    "role: "current role at issuance",
    "sid": "Session ID (links token to a session row for revocation checking)",
    "iat": "issued at",
    "exp": "Expiration",
    "iss": "Issuer (uri)",
    "aud": "audience (synkademy-api)"
    "kid": "Key ID of the signing key (required for key rotation)"
  }
```

Remove the `ver` claim. Without a defined semantics and enforcement path, it creates a false sense of revocation capability. If token schema versioning becomes necessary, introduce it explicitly at that time with a concrete invalidation mechanism.
Access token and session validation:
For ordinary API requests, validate the JWT signature and claims locally (no database hit). For high-sensitivity actions — password change, session revocation, payment operations — also query the session table to verify the session is still active. This provides a 15-minute revocation window for ordinary requests while giving you immediate revocation for critical paths.

### Refresh Token

- Format: 32 bytes of cryptographically random data, base64url-encoded (43 characters).
- Lifetime: 30 days (configurable per deployment).
- Transport: HttpOnly, Secure, SameSite=Strict cookie.
- Database storage: SHA-256 hash of the raw token only. Never store the raw token.
- Cookie path: /auth/ to limit scope.
___
Why this shape:

- The access token keeps API requests stateless and fast.
- The refresh session gives you revocation, device visibility, and token
  rotation.
- Keeping the refresh token in an `HttpOnly` cookie reduces XSS exposure.
- The authorization layer can read claims from the access token, but the source
  of truth for user status remains the database.

If the app later moves to enterprise SSO, this design can still support that by
swapping the credential verification step without changing the session model.

### JWT Signing Key Strategy
Use RS256 with an asymmetric key pair.

#### Initial setup:

1. Generate a 2048-bit RSA key pair (or an ES256 P-256 pair — equally secure and faster).
2. Store the private key in a secrets manager (Vault, AWS Secrets Manager, GCP Secret Manager — not in environment variables or source control).
3. Expose the public key at GET /.well-known/jwks.json (standard JWK Set format) for future resource server verification without shared secrets.
4. Include a kid (Key ID) field in every JWT header. The kid indexes into the JWK Set.

### Key rotation lifecycle:
Rotate signing keys every 90 days or immediately after any suspected compromise.

1. Generate new key pair. Assign a new kid.
2. Add the new public key to the JWK Set alongside the old one (dual-key period begins).
3. Begin signing new tokens with the new private key.
4. Old tokens signed with the previous key are still valid until they expire (15-minute window).
5. After 15 minutes, remove the old public key from the JWK Set.
6. Retire the old private key from the secrets manager.

This zero-downtime rotation requires no forced logouts.
___

## Refresh Token Rotation — Detailed Algorithm
This is the most security-critical implementation detail. Follow it exactly.

    ```
      FUNCTION refresh(incoming_cookie_token):

        1. Compute token_hash = SHA256(incoming_cookie_token)

        2. Query sessions WHERE refresh_token_hash = token_hash AND status != 'deleted'

        3. IF no session found:
            RETURN 401 Unauthorized  (unknown token — treat as attack)

        4. IF session.status == 'revoked':
            RETURN 401 Unauthorized

        5. IF session.status == 'rotated':
            // Token reuse detected. Revoke entire session family.
            UPDATE sessions SET status = 'revoked', revoked_at = NOW()
              WHERE family_id = session.family_id
            LOG event: refresh_reuse_detected (session_id, family_id, user_id, ip)
            RETURN 401 Unauthorized

        6. IF session.expires_at < NOW():
            UPDATE sessions SET status = 'revoked', revoked_at = NOW()
              WHERE id = session.id
            RETURN 401 Unauthorized  (expired — do not rotate)

        7. // Begin rotation — use a DB transaction
        BEGIN TRANSACTION:

          a. UPDATE sessions SET status = 'rotated' WHERE id = session.id AND status = 'active'
            // Optimistic lock: if this returns 0 rows, another request already rotated it
            IF rows_affected == 0:
              ROLLBACK
              RETURN 409 Conflict  (concurrent rotation — client should retry)

          b. new_raw_token = crypto_random_bytes(32)
          c. new_token_hash = SHA256(new_raw_token)

          d. INSERT INTO sessions (
              id, family_id, user_id, refresh_token_hash,
              status, expires_at, rotated_from_id, ip_address, user_agent, created_at, last_seen_at
            ) VALUES (
              new_uuid(), session.family_id, session.user_id, new_token_hash,
              'active', NOW() + 30 days, session.id, request.ip, request.user_agent, NOW(), NOW()
            )

          e. new_access_token = issue_jwt(user_id=session.user_id, sid=new_session.id)

        COMMIT

        8. Set-Cookie: refresh_token=<new_raw_token>; HttpOnly; Secure; SameSite=Strict; Path=/auth/; Max-Age=2592000
        9. RETURN { access_token: new_access_token, expires_in: 900 }

    ```
Race condition handling: Step 7a uses an optimistic lock on status = 'active'. If two concurrent refresh requests arrive with the same token, only one will update the row. The second sees rows_affected == 0 and returns 409, which the client can retry after a brief delay. Do not implement a grace window — it widens the reuse-detection gap.

___

## CSRF Defense
Decision: SameSite=Strict cookie + CORS allowlist.
Since the refresh cookie is scoped to SameSite=Strict and /auth/, cross-origin requests from third-party sites will not include the cookie, eliminating the primary CSRF vector. No synchronizer token or double-submit cookie is required as long as:

1. The cookie is marked SameSite=Strict (not Lax).
2. The CORS allowlist is enforced strictly on the server — only the known frontend origin(s) receive Access-Control-Allow-Origin.
3. State-changing requests require a Content-Type: application/json header (this is automatically enforced when the frontend uses fetch with a JSON body — HTML form submissions cannot set this header).

If the app ever needs cross-origin cookie delivery (e.g., a native mobile app sharing the web session endpoint), re-evaluate this decision and adopt double-submit cookie or a synchronizer token at that time.

___

## API Surface
### Public endpoints
- POST /auth/register (Rate limited by IP and emailPOST/auth/loginRate limited by IP and email)
- POST/auth/refreshRate (limited by IP; reads refresh cookie)
- POST/auth/logout (Reads refresh cookie; revokes session)
- POST/auth/forgot-password (Rate limited; always returns 200 to prevent enumeration)
- POST/auth/reset-passwordValidates (single-use token; invalidates all sessions)

### Authenticated endpoints
- GET/auth/me (current user profile)
- PATCH/auth/me (Updates display name etc.)
- POST/auth/change-password (Requires current password; invalidates all other sessions)
- GET/auth/sessions (Lists active sessions for the current user)
- DELETE/auth/sessions/:id (Revokes a specific session)
- DELETE/auth/sessions (Global logout — revokes all sessions)

### Future admin endpoints
- GET/admin/users
- PATCH/admin/users/:id/role
- PATCH/admin/users/:id/status

### Request and Response Shape

#### Login response
```
  {
    "user": {
      "id": "usr_01HX...",
      "email": "ada@example.com",
      "display_name": "Ada Lovelace",
      "role": "user",
      "email_verified": true
    },
    "access_token": "eyJhbGciOiJSUzI1NiIsImtpZCI6ImtleS0xIn0...",
    "expires_in": 900
  }
```
The refresh token is delivered via Set-Cookie only. It never appears in the JSON body.

#### Error shape
```
  {
    "error": {
      "code": "invalid_credentials",
      "message": "Invalid email or password."
    }
  }
```
Use the same invalid_credentials code and message for both wrong email and wrong password. Never distinguish them in the response.
___

## Authentication Flow
  ```
  LOGIN:
    Browser → POST /auth/login { email, password }
    Server  → verify password (Argon2id), check user.status == active
    Server  → issue JWT + create session row + set refresh cookie
    Browser ← { access_token, expires_in, user }

  AUTHENTICATED REQUEST:
    Browser → GET /api/resource  Authorization: Bearer <access_token>
    Server  → validate JWT signature + claims (no DB hit for ordinary requests)
    Browser ← protected resource

  TOKEN REFRESH:
    Browser → POST /auth/refresh  (refresh cookie sent automatically)
    Server  → rotation algorithm (see above)
    Browser ← { new access_token, expires_in }  + new refresh cookie

  LOGOUT:
    Browser → POST /auth/logout  (refresh cookie sent automatically)
    Server  → revoke session row, clear cookie (Max-Age=0)
    Browser ← 204 No Content
  ```
Frontend session management: The React app should track the access token expiry time in memory and proactively refresh 60 seconds before expiration using a setTimeout. On page load, immediately call POST /auth/refresh to bootstrap the session — do not persist the access token across page loads.
___

## Password Reset Flow
This flow is a frequent attack target. Specify every detail.

1. User submits POST /auth/forgot-password with their email.
2. Server always responds 200 OK with a generic message, regardless of whether the email exists.
3. If the email exists and the account is active, server generates a reset token:
  - 32 bytes of cryptographically random data, base64url-encoded.
  - Store only the SHA-256 hash in a password_reset_tokens table.
  - Expires in 60 minutes.
  - Mark as used = false.

4. Send the raw token to the user's email as a link: https://app.synkademy.com/reset-password?token=<raw>.
5. User submits POST /auth/reset-password with { token, new_password }.
6. Server hashes the incoming token and looks up the record. Reject if:
  - No matching hash found.
  - used == true.
  - expires_at < NOW().
  - All comparisons are constant-time.
7. On success:
  - Update password_hash.
  - Mark reset token as used = true.
  - Revoke all active sessions for this user.
  - Log the event.
8. Respond 200 OK. Do not auto-login — require the user to log in fresh.

### Password reset tokens table
id UUID
user_id UUID
token_hash text
expires_at timestamptz
used boolean
used_at timestamptz
created_at timestamptz
ip_address inet

### Security Requirements
#### Transport and browser

- Require HTTPS in production. Reject or redirect all HTTP.
- Refresh cookie: HttpOnly; Secure; SameSite=Strict; Path=/auth/; Max-Age=2592000.
- Set Strict-Transport-Security: max-age=63072000; includeSubDomains; preload on all responses.
- CORS: allowlist only known frontend origins. Reject preflight for unknown origins.
- Consider Content-Security-Policy headers on the React app to limit XSS blast radius.

#### Rate limiting and brute force
All limits should be enforced at both the IP level and the account level (where an account identifier is present in the request):

POST /auth/login (5 attempts per account per 15 minutes; 20 per IP per 15 minutes)
POST /auth/register (10 per IP per hour)
POST /auth/forgot-password (3 per email per hour; 20 per IP per hour)
POST /auth/reset-password (5 per token per hour)
POST /auth/refresh (60 per IP per minute)

Use sliding window rate limiting, not fixed window. Return 429 Too Many Requests with a Retry-After header.

### Timing side-channels and user enumeration

- Hash the password before checking whether the user exists. If the user does not exist, run Argon2id against a dummy hash to ensure consistent response time.
- POST /auth/forgot-password must return 200 OK regardless of whether the email is registered.
- All token comparisons (refresh_token_hash, reset_token_hash) must use crypto/subtle.ConstantTimeCompare in Go.
- Avoid per-account lockouts that reveal account existence — prefer per-account rate limiting with consistent error messages.

### Account lifecycle
- New accounts: status = pending_verification if email verification is required, active for invite-only flows.
- Password change: revoke all sessions except the current one (or all sessions — a product decision).
- Account compromise response: set status = disabled, revoke all sessions, optionally force a password reset.
- Deleted accounts: set status = deleted, revoke all sessions, preserve the row for audit purposes.
___
## Authorization Model

v1: Flat authorization
Every authenticated user has the same rights. Implement authorization as a policy layer from the start.

```
  // Middleware chain:
  // RequireAuth(next) — verifies JWT, attaches user context
  // Authorize("resource.action")(next) — evaluates policy

  // v1 policy: trivially grants all user-level actions to any authenticated user
  func (p *PolicyStore) Can(user *User, action string) bool {
      return user.Status == "active"
  }
```

Policy abstraction
Do not inline authorization decisions in route handlers. All authorization goes through Authorize(action, resource) middleware. This means adding RBAC in phase 4 requires updating the PolicyStore only — no route changes.
Suggested permission naming convention:

```
  auth.profile.read
  auth.profile.update
  auth.password.change
  auth.sessions.list
  auth.sessions.revoke
  meeting.create
  meeting.join
  meeting.moderate
  user.manage
  user.role.assign
```

### v1 → RBAC migration path
The migration is designed to be additive only — no destructive changes to the auth core.

### Phase 3 (authorization scaffolding):

- Persist the default user role in the users table (already present).
- Create permissions and role_permissions tables.
- Seed user role with all user-level permissions.
- PolicyStore reads from role_permissions instead of the hard-coded trivial grant.

### Phase 4 (RBAC expansion):

- Add instructor, host, student, admin roles.
- Create user_roles table for multi-role support (optional — start with single role per user if simpler).
- Add assignment UI and admin endpoints.
- Introduce resource_scopes if permissions become tenant- or meeting-scoped.

### Privilege escalation guardrails for RBAC:

- Role assignment requires explicit admin action — never derive roles from JWT claims alone (the database is the authority).
- When introducing new roles, default all existing users to user. Do not inherit elevated permissions automatically.
- The JWT role claim is a snapshot at issuance. For role-sensitive actions, re-read role from the database — do not trust the claim alone.
- Audit every role assignment and revocation event.
___

### Observability and Audit Logging

Log the following events at minimum. Store logs without raw tokens, passwords, or full email addresses (hash or truncate PII if the log store has weaker access controls than the database).

- user.registered user_id, ip
- user.email_verified user_id
- auth.login.success user_id, session_id, ip, user_agent
- auth.login.failureemail_hash, ip, reason
- auth.refresh.success user_id, session_id, ip
- auth.refresh.reuse_detectedf amily_id, session_id, ip
- auth.logout user_id, session_id, ip
- auth.logout.global user_id, session_count, ip
- auth.password.changed user_id, sessions_revoked
- auth.password.reset_requested user_id (if found), ip
- auth.password.reset_completed user_id, sessions_revoked
- auth.session.revoked user_id, session_id, revoked_by
- auth.token.invalid reason (expired, bad_signature, wrong_audience), ip
- admin.role.assigned target_user_id, role, assigned_by_user_id
- admin.user.status_changed target_user_id, old_status, new_status, changed_by

Log structured JSON to your observability stack. Set alerting on refresh_reuse_detected — it is a strong signal of session theft.
___

### Backend Package Structure (Go)

```
internal/
  auth/
    token.go          // JWT issue and validation
    password.go       // Argon2id hashing and verification
    session.go        // Session creation, rotation, revocation
    policy.go         // PolicyStore and Authorize logic
  users/
    model.go
    repository.go
  middleware/
    require_auth.go   // JWT validation, context attachment
    authorize.go      // Policy middleware wrapper
    rate_limit.go
  recovery/
    reset.go          // Password reset token lifecycle
    verify.go         // Email verification token lifecycle
  config/
    secrets.go        // Key loading from secrets manager
    settings.go
  repository/
    db.go             // Database connection and query helpers
```
Keeping auth logic in internal/auth makes it testable in isolation. The policy concern lives in internal/auth/policy.go — not in HTTP handlers — so RBAC is a policy change, not a routing change.

___
### Implementation Phases

#### Phase 1: Core authentication

- Register, login, refresh, logout.
- Argon2id password hashing.
- Session table with rotation and reuse detection (full algorithm above).
- GET /auth/me.
- Frontend: in-memory token storage, proactive refresh loop, session bootstrap on load.

#### Phase 2: Account recovery

- Password reset flow (full spec above).
- Email verification (or skip for invite-only, but keep the schema ready).
- Session invalidation on password change.

#### Phase 3: Authorization scaffolding

- Central PolicyStore with trivial grant.
- Authorize middleware wired to all routes.
- Permission tables seeded.

#### Phase 4: RBAC expansion

- Role assignment tools and admin views.
- Meeting and institution permission boundaries.
- Audit log alerting on privilege escalation events.

____

### Production Readiness Checklist

#### Infrastructure
[] HTTPS-only deployment with HSTS preload.
[] Secrets managed outside source control (Vault, AWS SM, GCP SM).
[] Database connection uses TLS.
[] Logs ship to an append-only store with restricted access.

#### Auth Core
[] Access token lifetime ≤ 15 minutes.
[] Refresh token rotation with reuse detection and family revocation.
[] All token and reset-code comparisons use crypto/subtle.ConstantTimeCompare.
[] Password hashing with Argon2id (tuned to ~200ms on production hardware).
[] forgot-password returns 200 regardless of email existence.
[] Login returns the same error for wrong email and wrong password.
[] Dummy Argon2id hash evaluated when email is not found (prevents timing leak).

#### Session management
[] Refresh cookie: HttpOnly; Secure; SameSite=Strict; Path=/auth/.
[] Server-side session revocation working.
[] Session revocation on password change.
[] Global logout endpoint tested.

#### Network and browser
[] CORS allowlist enforced — no wildcard in production.
[] Rate limiting on all auth endpoints (per-IP and per-account).
[] SameSite=Strict cookie verified with cross-site request test.

#### Observability
[] Structured audit log shipping all required events.
[] Alert configured on refresh_reuse_detected.
[] Alert configured on login failure spike (credential stuffing signal).

#### Testing
[] Login, refresh, logout happy paths.
[] Reuse detection: rotated token rejected + family revoked.
[] Race condition: concurrent refresh with same token → one succeeds, one 409.
[] Expired access token rejected.
[] Revoked session rejected within 15 minutes for high-sensitivity endpoints.
[] Password reset: used token rejected; expired token rejected.
[] Rate limiting triggers and resets correctly.
