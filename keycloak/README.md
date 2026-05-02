# Keycloak Setup Guide

Keycloak is the centralized identity provider for the Pred system. This folder contains all Keycloak-related configuration and documentation.

## Quick Start (5 minutes)

### 1. Install Dependencies

```bash
cd web-frontend
npm install next-auth
```

### 2. Start Keycloak

```bash
# From repo root
docker compose up -d keycloak postgres

# Wait for startup
sleep 30
```

Keycloak will be available at: **http://localhost:8080**

### 3. Configure Keycloak

See [CONFIGURATION.md](./CONFIGURATION.md) for detailed steps:

1. Login to Keycloak admin: `admin` / `changeme`
2. Create client: `web-frontend`
3. Get client secret from Credentials tab
4. Create test user: `testuser` / `Test123!`

### 4. Update Frontend Config

Copy client secret to `web-frontend/.env.local`:

```env
KEYCLOAK_CLIENT_SECRET=<paste-here>
```

### 5. Start Frontend and Test

```bash
cd web-frontend
npm run dev

# Visit http://localhost:3000 → Login → Dashboard
```

---

## Documentation Structure

| File | Purpose |
|------|---------|
| [README.md](./README.md) | This file — quick reference |
| [SETUP.md](./SETUP.md) | Detailed architecture & flow explanations |
| [CONFIGURATION.md](./CONFIGURATION.md) | Step-by-step Keycloak admin panel guide |
| [ARCHITECTURE.md](./ARCHITECTURE.md) | How everything works together |

---

## Architecture At A Glance

```
┌─────────────────────────────────────┐
│  Browser (User)                     │
│  http://localhost:3000              │
└──────────────┬──────────────────────┘
               │
         Login Request
               │
               ▼
┌─────────────────────────────────────┐
│  Web Frontend (Next.js)             │
│  NextAuth + Keycloak config         │
│  app/login → signIn("keycloak")     │
└──────────────┬──────────────────────┘
               │
         OAuth2 Flow
               │
               ▼
┌─────────────────────────────────────┐
│  Keycloak                           │
│  http://localhost:8080              │
│  - User authentication              │
│  - Token generation                 │
│  - Session management               │
└──────────────┬──────────────────────┘
               │
      Database operations
               │
               ▼
┌─────────────────────────────────────┐
│  PostgreSQL                         │
│  keycloak database                  │
│  (users, roles, tokens)             │
└─────────────────────────────────────┘
```

---

## File Structure

```
keycloak/
├── README.md                 ← This file (quick start)
├── SETUP.md                  ← Detailed setup guide
├── CONFIGURATION.md          ← Keycloak admin steps
├── ARCHITECTURE.md           ← How everything works
└── docker-compose-example.md ← Docker config reference
```

---

## Environment Variables

### For Web Frontend (`.env.local`)

Create this file in `web-frontend/`:

```env
NEXTAUTH_URL=http://localhost:3000
NEXTAUTH_SECRET=your-32-char-secret-here

KEYCLOAK_URL=http://localhost:8080
KEYCLOAK_REALM=master
KEYCLOAK_CLIENT_ID=web-frontend
KEYCLOAK_CLIENT_SECRET=<from-keycloak-admin>
```

See [.env.example.template](./env.example.template) in this folder for a reference copy.

---

## Key Files Created

| Location | File | Purpose |
|----------|------|---------|
| `web-frontend/` | `lib/auth.ts` | NextAuth + Keycloak config |
| `web-frontend/` | `app/login/page.tsx` | Login page |
| `web-frontend/` | `app/dashboard/page.tsx` | Protected dashboard |
| `web-frontend/` | `app/api/auth/[...nextauth]/route.ts` | OAuth callback handler |
| `web-frontend/` | `middleware.ts` | Route protection |
| `web-frontend/` | `app/providers.tsx` | Session wrapper |
| `docker-compose.yml` | `keycloak` service | Keycloak container config |

---

## Services and Ports

| Service | Port | URL |
|---------|------|-----|
| **Keycloak** | 8080 | http://localhost:8080 |
| **Postgres** | 5433 | localhost:5433 (database) |
| **Kafka** | 9092 | localhost:9092 (message broker) |
| **Web Frontend** | 3000 | http://localhost:3000 |
| **Event Processing API** | 8001 | http://localhost:8001 |

---

## Common Tasks

### Reset Keycloak Admin Password

```bash
docker exec keycloak /opt/keycloak/bin/kcadm.sh \
  update-user --username admin \
  --set password=newpassword123
```

### View Keycloak Logs

```bash
docker compose logs -f keycloak
```

### Restart Keycloak

```bash
docker compose restart keycloak
```

### Clear Keycloak Data

```bash
docker compose down -v keycloak  # Removes volume with all data
docker compose up -d keycloak    # Starts fresh
```

---

## Troubleshooting

### Keycloak won't start

Check if port 8080 is in use:
```bash
lsof -i :8080
```

### "Invalid client" error at login

- Verify `KEYCLOAK_CLIENT_ID` in `.env.local` matches client in Keycloak
- Check client exists in Keycloak admin panel
- Ensure client is enabled

### "Redirect URI mismatch" error

In Keycloak client settings:
- Add `http://localhost:3000/api/auth/callback/keycloak` to "Valid redirect URIs"
- Add `http://localhost:3000/login` to "Valid post logout redirect URIs"

### Session not persisting after page refresh

- Check `NEXTAUTH_SECRET` is set (32+ characters)
- Clear browser cookies and try again
- Make sure `.env.local` is loaded (restart `npm run dev`)

### Can't login with test user

- Go to Keycloak admin → Users → select user
- Check "Enabled" is ON
- Reset password from Credentials tab
- Make sure email is verified (for user creation)

---

## Next Steps

1. ✅ **Start infrastructure** → `docker compose up -d keycloak postgres`
2. ✅ **Configure Keycloak** → Follow [CONFIGURATION.md](./CONFIGURATION.md)
3. ✅ **Update frontend config** → Copy client secret to `.env.local`
4. ✅ **Test login** → `npm run dev` → http://localhost:3000 → Login
5. 🔲 **Integrate with Go services** → Validate JWT tokens in backend
6. 🔲 **Add multi-tenancy** → Include tenant_id in JWT claims
7. 🔲 **Email integration** → Wire up real SMTP for password resets

---

## Reference Documentation

- [Keycloak Official Docs](https://www.keycloak.org/docs/latest/)
- [OpenID Connect Protocol](https://openid.net/connect/)
- [NextAuth.js Documentation](https://next-auth.js.org/)
- [JWT Tokens Explained](https://jwt.io/)

---

## What's Keycloak?

Keycloak is an open-source **Identity and Access Management (IAM)** system. It:

- **Authenticates users** (username/password)
- **Generates JWT tokens** (for stateless API auth)
- **Manages sessions** (browser cookies)
- **Handles OAuth2/OIDC** (standard auth protocols)
- **Supports multiple realms** (separate workspaces)
- **Manages user attributes** (custom fields like tenant_id)

For your Pred system, it's the **single source of truth for user authentication**.

---

For detailed explanations, see [SETUP.md](./SETUP.md).
