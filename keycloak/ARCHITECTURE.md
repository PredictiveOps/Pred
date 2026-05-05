# Keycloak Architecture & Data Flow

Complete technical explanation of how Keycloak works within the Pred system.

## System Components

```
┌─────────────────────────────────────────────────────────────┐
│                      User's Browser                          │
│                   (localhost:3000)                           │
└──────────────────────────┬──────────────────────────────────┘
                           │
                    HTTP Requests
                           │
        ┌──────────────────┴──────────────────┐
        │                                     │
        ▼                                     ▼
┌──────────────────────┐          ┌──────────────────────┐
│   Next.js Frontend   │          │  Keycloak Server     │
│  (App Router)        │          │  (Identity Provider) │
│  NextAuth Enabled    │          │  Port: 8080          │
│  Port: 3000          │          │  OAuth2/OIDC         │
└──────────────────────┘          └──────┬───────────────┘
        ▲                                 │
        │                                 │
        └─────────────────────────────────┘
                  Session Cookie
                  (JWT in httpOnly)
                           │
                           ▼
                    ┌──────────────┐
                    │  PostgreSQL  │
                    │  keycloak DB │
                    │  (User Data) │
                    └──────────────┘
```

---

## Login Flow Step-by-Step

### **Phase 1: User Initiates Login**

```
User visits: http://localhost:3000
     ↓
app/page.tsx calls auth()
     ↓
No session exists
     ↓
Shows landing page with "Login" button
```

**What happens:**
- `auth()` function checks for existing session
- Looks in `httpOnly` cookie for session data
- If not found, returns `null`

### **Phase 2: User Clicks Login**

```
User clicks: "Login"
     ↓
Navigates to: http://localhost:3000/login
     ↓
app/login/page.tsx loads
     ↓
Shows "Login with Keycloak" button
```

**What happens:**
- Middleware checks session (none exists)
- Allows access to /login
- Page renders login button

### **Phase 3: User Clicks "Login with Keycloak"**

```
Button onClick: signIn("keycloak", { callbackUrl: "/dashboard" })
     ↓
NextAuth initiates OAuth2 Authorization Code Flow
     ↓
Generates state parameter (security)
     ↓
Redirects browser to Keycloak
```

**Request to Keycloak:**
```
GET http://localhost:8080/realms/master/protocol/openid-connect/auth?
    client_id=web-frontend
    &response_type=code
    &scope=openid profile email
    &redirect_uri=http://localhost:3000/api/auth/callback/keycloak
    &state=abc123xyz
```

**What happens:**
- NextAuth generates parameters
- Browser makes GET request to Keycloak
- Keycloak receives the request

### **Phase 4: User Authenticates**

```
Browser loads: Keycloak login page (http://localhost:8080)
     ↓
User enters: username = testuser
User enters: password = Test123!
User clicks: Login button
     ↓
Keycloak validates credentials
     ↓
Credentials match in keycloak database
     ↓
Authentication successful
```

**What happens:**
- Keycloak queries PostgreSQL for user `testuser`
- Verifies password hash matches
- Checks if user is enabled
- Checks if user has required realm access

### **Phase 5: Authorization Code Generated**

```
Keycloak generates:
  - Authorization Code (short-lived, single-use)
  - Session in Keycloak (for SSO)
     ↓
Keycloak redirects browser
```

**Redirect to:**
```
http://localhost:3000/api/auth/callback/keycloak?
    code=1234567890abcdef
    &state=abc123xyz
```

**What happens:**
- Authorization code is valid for 10 minutes
- Can only be used once
- Tied to the original request

### **Phase 6: NextAuth Receives Authorization Code**

```
Browser receives redirect with code
     ↓
Browser follows redirect to: /api/auth/callback/keycloak?code=...
     ↓
NextAuth route handler receives request
     ↓
Verifies state parameter matches (security check)
```

**What happens:**
- Browser automatically follows the redirect
- NextAuth middleware intercepts
- Extracts authorization code from URL
- Verifies state to prevent CSRF attacks

### **Phase 7: Code-to-Token Exchange**

```
NextAuth (on server) makes secure request to Keycloak:
     ↓
POST http://localhost:8080/realms/master/protocol/openid-connect/token
     ↓
Headers:
  - Content-Type: application/x-www-form-urlencoded
Body:
  - grant_type=authorization_code
  - code=1234567890abcdef
  - client_id=web-frontend
  - client_secret=super-secret-key-from-keycloak
  - redirect_uri=http://localhost:3000/api/auth/callback/keycloak
```

**What happens:**
- **Server-to-server** request (not visible in browser)
- NextAuth proves identity with `client_secret` (never sent to browser)
- Keycloak validates the request
- Verifies authorization code hasn't been used
- Verifies redirect_uri matches

### **Phase 8: Keycloak Issues Tokens**

```
Keycloak validates everything
     ↓
Generates JWT tokens:
  1. Access Token (for API calls)
  2. Refresh Token (for getting new access token)
  3. ID Token (for user info)
```

**Example JWT Token (decoded):**
```json
{
  "iss": "http://localhost:8080/realms/master",
  "sub": "user-uuid-here",
  "aud": "web-frontend",
  "exp": 1609459200,
  "iat": 1609455600,
  "name": "Test User",
  "email": "test@example.com",
  "preferred_username": "testuser"
}
```

**Response to NextAuth:**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "id_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_expires_in": 86400
}
```

**What happens:**
- Keycloak creates tokens (cryptographically signed)
- Access token expires in 1 hour
- Refresh token expires in 24 hours
- Tokens contain user information (claims)

### **Phase 9: NextAuth Processes Tokens**

```
NextAuth receives tokens
     ↓
Callbacks in lib/auth.ts execute:
  1. jwt callback: Extracts tokens
  2. session callback: Adds tokens to session
     ↓
NextAuth creates encrypted session cookie
```

**JWT Callback:**
```typescript
async jwt({ token, account }) {
  if (account) {
    token.accessToken = account.access_token;    // Store for later use
    token.refreshToken = account.refresh_token;
  }
  return token;
}
```

**Session Callback:**
```typescript
async session({ session, token }) {
  session.accessToken = token.accessToken;  // Make available to app
  return session;
}
```

**What happens:**
- NextAuth extracts useful data from JWT
- Stores everything in an encrypted session object
- Session is placed in `httpOnly` cookie (secure, can't be accessed by JavaScript)

### **Phase 10: Session Cookie Set**

```
NextAuth sends response to browser:
  - Set-Cookie: next-auth.session-token=encrypted-session-data; HttpOnly; Secure
     ↓
Browser stores cookie
     ↓
NextAuth redirects to: /dashboard
```

**What happens:**
- Cookie is `httpOnly` (JavaScript can't access it)
- Cookie is `Secure` (only sent over HTTPS in production)
- Cookie contains encrypted session data
- Includes user info, tokens, expiration time

### **Phase 11: Dashboard Loads**

```
Browser navigates to: http://localhost:3000/dashboard
     ↓
Middleware.ts runs:
  - Calls auth()
  - Session cookie exists
  - Middleware allows request
     ↓
app/dashboard/page.tsx loads
     ↓
useSession() hook retrieves session
     ↓
Dashboard renders with user info
```

**What happens:**
- Browser automatically sends session cookie with request
- Server validates cookie signature
- Session data is decrypted
- User info is available in component

### **Phase 12: User Logged In!**

```
Dashboard displays:
  - Welcome message with username
  - User email
  - Logout button
```

**Session Details:**
```typescript
{
  user: {
    name: "Test User",
    email: "test@example.com"
  },
  accessToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  expires: "2026-05-01T10:00:00Z"
}
```

---

## What Happens on Page Refresh

```
User refreshes page (F5)
     ↓
Middleware.ts runs
     ↓
auth() function checks for session cookie
     ↓
Cookie exists and hasn't expired
     ↓
Session is valid
     ↓
Page loads instantly (no need to login again)
```

**Why session persists:**
- Session cookie is stored on disk (in browser)
- Cookie is automatically sent with every request
- NextAuth validates the cookie signature
- If valid and not expired, user stays logged in

---

## What Happens on Logout

```
User clicks: Logout button
     ↓
onClick: signOut({ callbackUrl: "/login" })
     ↓
NextAuth clears session cookie
     ↓
NextAuth redirects to: /login
```

**What happens:**
- Session cookie is deleted (Set-Cookie with empty value)
- User is immediately logged out
- Can't access /dashboard anymore (redirected to /login)

---

## Token Refresh Flow

When access token expires (after 1 hour):

```
Frontend makes API call:
  - Sends expired access token
     ↓
Backend API rejects (token expired)
     ↓
NextAuth detects expired token
     ↓
NextAuth sends refresh token to Keycloak:
  POST /protocol/openid-connect/token
  grant_type=refresh_token
  refresh_token=...
     ↓
Keycloak validates refresh token
     ↓
Keycloak issues new access token
     ↓
NextAuth stores new token
     ↓
NextAuth retries original API call
```

**Benefits:**
- User doesn't see login prompt if active
- Access tokens are short-lived (secure)
- Refresh tokens are longer-lived (convenient)

---

## Security Features

### **PKCE (Proof Key for Code Exchange)**

For web apps, adds extra security:

1. Client generates random string (code_verifier)
2. Hashes it (code_challenge)
3. Sends code_challenge in authorization request
4. When exchanging code, proves it knows code_verifier
5. Prevents code interception attacks

NextAuth handles this automatically.

### **State Parameter**

Prevents CSRF (Cross-Site Request Forgery):

1. NextAuth generates random state
2. Sends state in authorization request
3. User redirected back with same state
4. NextAuth verifies state matches
5. If state doesn't match → reject (attack detected)

### **HttpOnly Cookies**

Session cookie is `httpOnly`:
- JavaScript can't access it (prevents XSS attacks)
- Browser automatically sends with requests
- More secure than storing token in localStorage

### **Cryptographic Signatures**

JWT tokens are signed:
- Only Keycloak can create valid tokens (private key)
- Frontend can verify signatures (public key)
- If token is tampered with, signature is invalid

---

## Multi-Tenant Support (Future)

When adding multi-tenancy, add to Keycloak:

```
User Attributes:
  tenant_id = "acme-corp"
```

Token Mapper (in Keycloak):
```
Claim Name: tenant_id
User Attribute: tenant_id
→ JWT token will include: "tenant_id": "acme-corp"
```

Frontend/Backend usage:
```typescript
const session = await auth();
const tenantId = session.user.tenant_id;  // "acme-corp"

// Filter data by tenant
const events = await getEvents(tenantId);
```

---

## Error Scenarios

### **Invalid Credentials**

```
User enters wrong password
     ↓
Keycloak queries database
     ↓
Password hash doesn't match
     ↓
Keycloak returns error page
     ↓
User can try again
```

### **Expired Code**

```
User takes >10 minutes to login
     ↓
Authorization code expires
     ↓
NextAuth exchanges code
     ↓
Keycloak rejects (expired)
     ↓
NextAuth redirects to login
     ↓
User must start over
```

### **Expired Session**

```
Session cookie expires (default: 30 days)
     ↓
User visits app
     ↓
Middleware checks session
     ↓
Session is invalid
     ↓
Redirected to /login
     ↓
User must login again
```

### **Invalid Client Secret**

```
NextAuth uses wrong client_secret in token exchange
     ↓
Keycloak rejects request
     ↓
NextAuth fails
     ↓
Login fails with "Invalid client" error
```

---

## Configuration: How It All Connects

**docker-compose.yml:**
```yaml
keycloak:
  KC_DB_URL: jdbc:postgresql://postgres:5432/keycloak
  KC_HOSTNAME: localhost
  KC_HOSTNAME_PORT: 8080
```

**web-frontend/.env.local:**
```env
KEYCLOAK_URL=http://localhost:8080
KEYCLOAK_CLIENT_ID=web-frontend
KEYCLOAK_CLIENT_SECRET=<secret>
```

**web-frontend/lib/auth.ts:**
```typescript
KeycloakProvider({
  clientId: process.env.KEYCLOAK_CLIENT_ID,
  clientSecret: process.env.KEYCLOAK_CLIENT_SECRET,
  issuer: `${process.env.KEYCLOAK_URL}/realms/master`,
})
```

**All pieces connect:**
- Keycloak runs on port 8080
- Postgres stores Keycloak data
- NextAuth knows where to find Keycloak
- NextAuth proves identity with client secret
- Everything uses configured URLs

---

## Performance Considerations

| Operation | Time | Notes |
|-----------|------|-------|
| Login | 1-2s | Depends on network |
| Token Exchange | <500ms | Server-to-server |
| Session Validation | <10ms | Local cookie check |
| Token Refresh | <500ms | Only when needed |
| Page Load (logged in) | <1s | No login needed |

---

## Summary

1. **User clicks login** → Redirected to Keycloak
2. **User authenticates** → Keycloak verifies password
3. **Authorization code sent** → Keycloak redirects back with code
4. **Code exchanged for tokens** → NextAuth uses client secret
5. **Session created** → Secure httpOnly cookie
6. **Page loads** → Session found in cookie
7. **User stays logged in** → Cookie sent with every request
8. **Token expires after 1 hour** → Refresh token gets new one
9. **User logs out** → Cookie deleted

This flow is industry-standard (OAuth2/OIDC) and used by GitHub, Google, Microsoft, etc.

---

See [README.md](./README.md) for quick start and [CONFIGURATION.md](./CONFIGURATION.md) for admin steps.
