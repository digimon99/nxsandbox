# NX Sandbox — Product Requirements Document

> **Version:** 1.0.0  
> **Status:** Draft  
> **License:** MIT  
> **Repository:** [github.com/digimon99/nxsandbox](https://github.com/digimon99/nxsandbox)  
> **Primary Stack:** Go, React, PostgreSQL, Bubblewrap (bwrap)  
> **Auth Model:** OTP + JWT (Supabase-inspired, Gmail-resilient session handling)

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Product Vision & Positioning](#2-product-vision--positioning)
3. [Architecture Overview](#3-architecture-overview)
4. [Technology Choices](#4-technology-choices)
5. [Authentication & Session Model](#5-authentication--session-model)
6. [SSR Pages — Public Surface](#6-ssr-pages--public-surface)
7. [React Dashboard — Authenticated Surface](#7-react-dashboard--authenticated-surface)
8. [CLI — `nx` Client Tool](#8-cli--nx-client-tool)
9. [Sandbox Engine — bwrap + Postgres](#9-sandbox-engine--bwrap--postgres)
10. [API Design](#10-api-design)
11. [Database Schema](#11-database-schema)
12. [Styling & Design System](#12-styling--design-system)
13. [SEO Strategy](#13-seo-strategy)
14. [Deployment & Packaging](#14-deployment--packaging)
15. [Open Source Strategy](#15-open-source-strategy)
16. [Milestones & Roadmap](#16-milestones--roadmap)
17. [Appendix — Competitive Context](#17-appendix--competitive-context)

---

## 1. Executive Summary

**NX Sandbox** is an open-source Platform-as-a-Service that flips the traditional deployment model: instead of pushing source code and waiting for a remote build, developers push **pre-built binaries** into lightweight, isolated sandboxes. Every sandbox gets a colocated PostgreSQL database with sub-millisecond latency. No builds on deploy. No lock-in. No Docker daemon.

### The One-Liner

> Costs like a VPS, performs like a dedicated server, deploys like a PaaS.

### Core Differentiators

| Differentiator | NX Sandbox | Vercel | Railway | Fly.io |
|---------------|------------|--------|---------|--------|
| Build model | **Pre-built binary** | Git push → build | Git push → build | Dockerfile → build |
| Isolation | bwrap (~5 MB overhead) | Firecracker microVM | Docker container | Firecracker microVM |
| Database | **Colocated Postgres, sub-ms** | 3rd-party (Neon, 10-50ms) | Shared Postgres (1-5ms) | Remote Postgres (1-10ms) |
| Lock-in | **Zero — pg_dump → leave** | High (Edge Functions, KV) | Medium | Medium |
| License | **MIT (self-hostable)** | Proprietary | Proprietary | Proprietary |
| Cold start | **<100ms** (no container pull) | 200-500ms | 1-3s | 300ms-3s |

---

## 2. Product Vision & Positioning

### 2.1 Target Audience

**Primary:** Indie developers and small teams who:
- Already build locally (Go, Rust, Bun, Zig binaries)
- Want Vercel-like simplicity without Vercel lock-in
- Need a real database, not a serverless wrapper
- Value portability above all — pg_dump and leave, anytime

**Secondary:** Self-hosters who run Coolify/Dokploy but want:
- Lighter footprint (bwrap vs Docker)
- Faster deploys (no image build step)
- Cleaner multi-app management

**Tertiary:** Open-source projects needing free/self-hosted staging environments.

### 2.2 Positioning Statement

> NX Sandbox is what you get when you cross a $5 VPS price point with Vercel's deployment workflow — minus the builds, minus the lock-in, plus a real database. It's the only managed platform that accepts pre-built binaries, and the only self-hostable PaaS using bwrap instead of Docker.

### 2.3 Anti-Positioning

- **NOT** a CI/CD platform — build happens elsewhere (local, GitHub Actions, whatever)
- **NOT** a container registry — binaries are pushed directly, not wrapped in OCI images
- **NOT** multi-tenant at launch — single-tenant bwrap model. gVisor/Firecracker tier later for multi-tenant hosting

---

## 3. Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      NX HOST (Single Binary)                 │
│                                                              │
│  ┌──────────┐  ┌───────────┐  ┌──────────┐  ┌───────────┐  │
│  │ SSR      │  │ REST API  │  │ React    │  │ Admin     │  │
│  │ Pages    │  │ (Go)      │  │ Dashboard│  │ Dashboard │  │
│  │ (Go)     │  │ :8080     │  │ SPA      │  │ (React)   │  │
│  └──────────┘  └───────────┘  └──────────┘  └───────────┘  │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              Sandbox Manager (Go)                     │   │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐              │   │
│  │  │ bwrap   │  │ bwrap   │  │ bwrap   │  ... N apps  │   │
│  │  │ app:3001│  │ app:3002│  │ app:3003│              │   │
│  │  └─────────┘  └─────────┘  └─────────┘              │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
│  ┌──────────────────────┐  ┌──────────────────────┐         │
│  │ PostgreSQL Cluster   │  │ SFTP Service (Go)    │         │
│  │ ┌────┐┌────┐┌────┐  │  │ Binary uploads       │         │
│  │ │ DB1││ DB2││ DBn│  │  │ DB backup downloads   │         │
│  │ └────┘└────┘└────┘  │  │ File management      │         │
│  └──────────────────────┘  └──────────────────────┘         │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │           Cloudflare DNS Integration (Go)             │   │
│  │   *.nxapp.dev wildcard → sandbox routing              │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 3.1 Single Binary Principle

**Everything ships as one Go binary.** The binary embeds:
- All SSR HTML templates (Go `embed.FS`)
- React dashboard production build (embedded via `embed.FS`)
- Static assets (fonts, favicon, robots.txt)
- TailwindCSS compiled stylesheets (pre-built, inlined)
- Database migrations (embedded SQL files)
- Default configuration

```go
//go:embed all:templates
var templatesFS embed.FS

//go:embed all:static
var staticFS embed.FS

//go:embed all:dashboard/dist
var dashboardFS embed.FS

//go:embed all:migrations
var migrationsFS embed.FS
```

**Result:** One file. `scp` it. Run it. Done.

### 3.2 Port Allocation

| Service | Port | Notes |
|---------|:----:|------|
| HTTP Server (SSR + API + Dashboard) | 8080 | Single entry point |
| Admin API (internal) | 8081 | Loopback only |
| SFTP Service | 2222 | Binary uploads |
| App Sandboxes | 9000-9999 | Dynamic allocation, 1000 ports per host |
| PostgreSQL | 5432 | Standard |

---

## 4. Technology Choices

### 4.1 Go — Primary Language

**Decision:** Go for the entire backend, SSR, API, CLI, and sandbox daemon.

**Rationale:**

| Factor | Go | Rust | Assessment |
|--------|:--:|:----:|------------|
| Team proficiency | ✅ Existing stack | ❌ Learning curve | Go ships tomorrow |
| Build speed | ✅ 3-5 seconds | ❌ 2-5 minutes | Fast iteration > micro-optimization |
| Binary size | ⚠️ ~8-15 MB | ✅ ~1-3 MB | Acceptable for a server binary |
| Memory usage | ⚠️ GC overhead | ✅ Zero-cost | GC-tuned Go is within 20% of Rust for server workloads |
| Ecosystem fit | ✅ Caddy, Traefik, Kubernetes | ⚠️ Growing but smaller | Cloud-native = Go-native |
| Concurrency | ✅ Goroutines | ✅ async/await | Both excellent |
| Talent pool | ✅ 2-3M developers | ⚠️ ~500K | 6x larger hiring pool |
| bwrap integration | ✅ `os/exec` native | ✅ `std::process` | Both trivial |
| `embed.FS` | ✅ Native since Go 1.16 | ❌ `rust-embed` crate | Go's embed is first-class |
| Cross-compilation | ✅ `GOOS/GOARCH` | ✅ `cross` / `cargo` | Both excellent |

**Verdict:** Go for v1.0 through v3.0. Revisit Rust for the sandbox daemon at 1,000+ customers when the micro-optimizations (binary size, memory, cold start) justify the rewrite cost. Fly.io followed this exact path — Go service mesh early, Rust proxy later.

### 4.2 React — Dashboard

**Decision:** React 19 with React Router 7, TailwindCSS v4, Vite.

**Rationale:**
- Largest ecosystem, most developers
- SPA mode — dashboard doesn't need SSR (it's behind auth)
- Vite for fast builds (~2s prod build)
- Embedded into Go binary at build time

### 4.3 PostgreSQL — Database

**Decision:** Single PostgreSQL cluster, per-sandbox databases.

```
PostgreSQL Cluster
├── nxsandbox       ← Host metadata (users, apps, tokens, audit)
├── sandbox_app_1   ← User's app database
├── sandbox_app_2   ← User's app database
└── sandbox_app_n   ← ...
```

**Why shared cluster, not per-sandbox Postgres process:**
- 100 sandboxes × 50-100MB per Postgres process = 5-10GB RAM just for idle databases
- Shared cluster: ~200MB for connection pool + per-database overhead (~5MB each)
- Dramatically better density per host

Each sandbox app connects via `localhost:5432/sandbox_app_N` with unique credentials. Latency is sub-millisecond (Unix socket or localhost TCP).

### 4.4 Bubblewrap (bwrap) — Sandbox Isolation

**Decision:** bwrap for single-tenant isolation. gVisor/Firecracker as future tier.

**bwrap profile per sandbox:**

```bash
bwrap \
  --ro-bind /usr /usr \
  --ro-bind /lib /lib \
  --ro-bind /lib64 /lib64 \
  --ro-bind /bin /bin \
  --tmpfs /tmp \
  --proc /proc \
  --dev /dev \
  --bind /data/sandbox/$APP_ID /app \
  --unshare-user \
  --unshare-ipc \
  --unshare-pid \
  --unshare-net \
  --hostname sandbox-$APP_ID \
  /app/binary
```

| Isolation Flag | What It Blocks |
|----------------|----------------|
| `--unshare-user` | Cannot see host users, cannot `sudo` |
| `--unshare-ipc` | Cannot see host IPC namespace |
| `--unshare-pid` | Own PID namespace, cannot see host processes |
| `--unshare-net` | Own network namespace, cannot sniff host traffic |
| `--tmpfs /tmp` | Ephemeral temp, wiped on restart |
| `--ro-bind /usr` | Read-only system binaries, cannot modify |

**Critical security note:** bwrap shares the host kernel. A kernel vulnerability = escape. For single-tenant usage (one org hosting their own apps), this is acceptable — the same risk exists for any process on the host. For multi-tenant (hosting untrusted third-party code), upgrade to gVisor or Firecracker.

**Reference:** Claude Code CLI uses bwrap for sandboxed command execution (Anthropic, 2025). The approach is production-validated at scale.

---

## 5. Authentication & Session Model

### 5.1 Design Philosophy: Gmail-Resilient Sessions

Inspired by Supabase `gotrue` and Gmail's session handling:

> **Never clear session data unless the user explicitly signs off.**

This means sessions survive:
- ✅ Network loss (WiFi → cellular → offline → back)
- ✅ Browser tab close and reopen
- ✅ Device sleep and wake
- ✅ Client-side JavaScript errors
- ✅ Page crash and reload
- ✅ Browser restart
- ✅ 30 days of inactivity (refresh token window)

### 5.2 Token Architecture

```
┌──────────────────────────────────────────────────────┐
│                  AUTH TOKEN FLOW                      │
│                                                       │
│  User enters email → OTP sent → OTP verified          │
│         │                                              │
│         ▼                                              │
│  ┌─────────────────────────────────────┐              │
│  │  localStorage (Client-side JS)       │              │
│  │                                       │              │
│  │  auth_token          JWT (15 min)    │              │
│  │  auth_token_expires   Unix timestamp  │              │
│  │  auth_user           JSON user obj   │              │
│  └─────────────────────────────────────┘              │
│                                                       │
│  ┌─────────────────────────────────────┐              │
│  │  HttpOnly Cookie (Server-set)        │              │
│  │                                       │              │
│  │  refresh_token       JWT (30 days)   │              │
│  │  path=/                              │              │
│  │  secure=true                         │              │
│  │  samesite=strict                     │              │
│  │  httpOnly=true                       │              │
│  └─────────────────────────────────────┘              │
└──────────────────────────────────────────────────────┘
```

### 5.3 Token Lifetimes

| Token | Location | Lifetime | Renewal |
|-------|----------|:--------:|---------|
| `auth_token` (access) | `localStorage` | 15 minutes | Auto-refresh via `refresh_token` cookie |
| `refresh_token` | HttpOnly cookie | 30 days | Rotated on each use |
| OTP code | Email | 10 minutes | Single use, invalidated after verification |

### 5.4 Storage Key Names (Exact)

```typescript
// These names are EXACT — no deviation allowed
localStorage.setItem('auth_token', accessToken);
localStorage.setItem('auth_token_expires', String(expiresAtUnix));
localStorage.setItem('auth_user', JSON.stringify(user));

// Cookie (set by server, read-only from JS perspective)
// Name: refresh_token
// Value: <JWT>
// Max-Age: 2592000 (30 days)
// Path: /
// HttpOnly: true
// Secure: true
// SameSite: Strict
```

### 5.5 Auto-Refresh Protocol

```
┌─────────────────────────────────────────────────┐
│  Every API request:                              │
│                                                   │
│  1. Read auth_token from localStorage             │
│  2. Check auth_token_expires                      │
│     │                                              │
│     ├─ Valid (> 5 min remaining) → use it         │
│     │                                              │
│     ├─ Expiring (< 5 min) → POST /api/auth/refresh│
│     │   ├─ Success → update localStorage, proceed │
│     │   └─ Fail (refresh also expired) →          │
│     │       redirect to /signin,                   │
│     │       BUT keep stored email/path for restore │
│     │                                              │
│     └─ Missing → try refresh_token cookie          │
│         ├─ Success → repopulate localStorage       │
│         └─ Fail → redirect to /signin              │
└─────────────────────────────────────────────────┘
```

### 5.6 Resilience Patterns

```typescript
// Client-side resilience: NEVER clear session on error
// Only clear on explicit signoff

// ❌ NEVER DO THIS:
catch (error) {
  localStorage.clear();  // WRONG — user loses session
  window.location = '/signin';
}

// ✅ ALWAYS DO THIS:
catch (error) {
  // Retry with exponential backoff
  if (retries < 3) {
    await delay(2 ** retries * 1000);
    return tryRefresh();
  }
  // Show degraded UI, keep session data intact
  showOfflineBanner();
  // User's session is preserved — they can retry
}
```

### 5.7 Session Clear — Only These Events

| Event | Clears Session? |
|-------|:---:|
| User clicks "Sign Off" | ✅ Yes |
| User explicitly signs out on another device | ✅ Yes (server revoke) |
| Refresh token expires (30 days) | ✅ Yes (server-side) |
| Network error | ❌ No — retry |
| JS error | ❌ No — preserve |
| Tab close | ❌ No — localStorage persists |
| Device sleep | ❌ No — timers recover |
| Page crash | ❌ No — reload and refresh |
| 401 from server | ⚠️ Try refresh, only clear if refresh also fails |

### 5.8 OTP Flow (Supabase-Inspired)

```
POST /api/auth/signin
  Body: { "email": "user@example.com" }
  → Server generates 6-digit OTP
  → Stores hash in DB with 10-min TTL
  → Sends email via Resend/SES
  → Returns: { "success": true, "message": "Check your email" }

POST /api/auth/verify
  Body: { "email": "user@example.com", "otp": "123456" }
  → Server validates OTP hash
  → Creates user if first sign-in (auto-registration)
  → Issues auth_token + refresh_token
  → Sets refresh_token HttpOnly cookie
  → Returns: { "auth_token": "...", "expires_at": 1234567890, "user": {...} }
  → Client stores in localStorage (exact key names)

GET /api/auth/verify?token=<jwt>&email=<urlencoded>
  → Magic link alternative (email deep link)
  → Same as OTP verify, but via GET with token param
  → Redirects to /verified?token=...&email=...
  → /verified page extracts token, stores in localStorage, redirects to /dashboard
```

---

## 6. SSR Pages — Public Surface

All SSR pages are Go-rendered with `html/template`. No JavaScript framework. Fully functional without JS.

### 6.1 Route Map

| Route | Method | Purpose | Auth |
|-------|:------:|---------|:----:|
| `/` | GET | Home / landing page | Public |
| `/signin` | GET/POST | Email input → send OTP | Public |
| `/signup` | GET | Same as signin (auto-registration) | Public |
| `/signin/verify` | GET/POST | OTP code input → verify | Public |
| `/verified` | GET | Magic link landing, extract token → localStorage → redirect | Public |
| `/signoff` | GET | Clear session, redirect to `/` | Public |
| `/docs` | GET | Documentation (embedded markdown) | Public |
| `/pricing` | GET | Pricing page | Public |

### 6.2 Home Page (`/`)

**Go template:** `templates/pages/home.html`

**Sections:**
1. **Hero** — "Deploy Without Builds. Own Your Infrastructure." + CTA
2. **How It Works** — 3 steps: Build → Push → Promote (animated sequence)
3. **Comparison Table** — NX vs Vercel vs Railway vs Fly.io
4. **Code Block** — `nx push ./myapp` terminal demo with syntax highlighting
5. **Testimonials / Use Cases** — Placeholder for launch
6. **Footer** — GitHub link, license, docs

**SEO Metadata:**
```html
<title>NX Sandbox — Deploy Pre-Built Binaries. No Builds. No Lock-In.</title>
<meta name="description" content="Open-source PaaS that deploys your pre-built binaries into isolated sandboxes with colocated PostgreSQL. MIT licensed. Self-hostable." />
<link rel="canonical" href="https://nxsandbox.com" />
<meta property="og:title" content="NX Sandbox — Deploy Pre-Built Binaries. No Builds. No Lock-In." />
<meta property="og:description" content="The only PaaS that accepts pre-built binaries. bwrap isolation. Colocated Postgres. Zero lock-in." />
```

### 6.3 Sign In Page (`/signin`)

**Go template:** `templates/pages/signin.html`

**Zero JavaScript required.** Uses `<form method="POST">` with progressive enhancement.

```html
<form method="POST" action="/signin" id="signin-form">
  <label for="email">Email address</label>
  <input 
    type="email" 
    id="email" 
    name="email" 
    required 
    autocomplete="email"
    placeholder="you@example.com"
  />
  <button type="submit">Send verification code</button>
  <p>No password needed. We'll email you a code.</p>
</form>
```

**Progressive enhancement (optional JS):**
- Client-side email validation before submit
- Disable button on submit (prevent double-send)
- Show countdown for resend (60 seconds)

### 6.4 Verify Page (`/signin/verify`)

**Go template:** `templates/pages/verify.html`

```html
<form method="POST" action="/signin/verify" id="verify-form">
  <input type="hidden" name="email" value="{{.Email}}" />
  <label for="otp">Verification code</label>
  <input 
    type="text" 
    id="otp" 
    name="otp" 
    required 
    inputmode="numeric"
    pattern="[0-9]{6}"
    maxlength="6"
    autocomplete="one-time-code"
    placeholder="000000"
  />
  <button type="submit">Verify & Sign In</button>
  <p>Code expires in 10 minutes. <a href="/signin">Send a new code</a></p>
</form>
```

### 6.5 Verified Page (`/verified`)

**Go template:** `templates/pages/verified.html`

This is the magic link landing page. It receives `?token=...&email=...` and:

1. Server validates the token
2. Renders a page that writes to `localStorage` via inline `<script>`
3. Redirects to `/dashboard` after 1 second

```html
<script>
  (function() {
    const params = new URLSearchParams(window.location.search);
    const token = params.get('token');
    const email = params.get('email');
    
    if (!token || !email) {
      window.location.href = '/signin';
      return;
    }
    
    // Parse JWT to get expiration
    const payload = JSON.parse(atob(token.split('.')[1]));
    
    localStorage.setItem('auth_token', token);
    localStorage.setItem('auth_token_expires', String(payload.exp));
    localStorage.setItem('auth_user', JSON.stringify(payload.user));
    
    // Clean URL (remove token from browser history)
    window.history.replaceState({}, '', '/verified');
    
    // Redirect to dashboard
    setTimeout(() => {
      window.location.href = '/dashboard';
    }, 800);
  })();
</script>
```

### 6.6 Sign Off Page (`/signoff`)

**Go template:** `templates/pages/signoff.html`

- Clears `refresh_token` cookie (server-side `Set-Cookie` with `Max-Age=0`)
- Clears `localStorage` via inline script
- Redirects to `/`

```html
<script>
  localStorage.removeItem('auth_token');
  localStorage.removeItem('auth_token_expires');
  localStorage.removeItem('auth_user');
  window.location.href = '/';
</script>
```

---

## 7. React Dashboard — Authenticated Surface

### 7.1 Route Map

| Route | Component | Purpose |
|-------|-----------|---------|
| `/dashboard` | `DashboardHome` | Overview: apps, usage, quick actions |
| `/dashboard/apps` | `AppsList` | List all sandboxes |
| `/dashboard/apps/new` | `AppCreate` | Push new binary → create sandbox |
| `/dashboard/apps/:id` | `AppDetail` | Single app: logs, metrics, preview URLs |
| `/dashboard/apps/:id/settings` | `AppSettings` | Env vars, domain, scaling |
| `/dashboard/apps/:id/database` | `AppDatabase` | DB backup, restore, connection string |
| `/dashboard/apps/:id/deployments` | `DeploymentHistory` | Version history, rollback |
| `/dashboard/settings` | `UserSettings` | Profile, email preferences |
| `/dashboard/admin/*` | `Admin*` | **Role: admin** — host management |
| `/dashboard/admin/users` | `AdminUsers` | User management |
| `/dashboard/admin/host` | `AdminHost` | Host metrics, Postgres status |
| `/dashboard/admin/audit` | `AdminAudit` | Audit logs |

### 7.2 Route Guard Pattern

```typescript
// ProtectedRoute.tsx
import { Navigate, useLocation } from 'react-router-dom';

function ProtectedRoute({ children, requiredRole }: { 
  children: React.ReactNode; 
  requiredRole?: 'admin';
}) {
  const location = useLocation();
  const token = localStorage.getItem('auth_token');
  const user = JSON.parse(localStorage.getItem('auth_user') || 'null');
  const expires = Number(localStorage.getItem('auth_token_expires'));
  
  // Check token existence and expiration
  if (!token || !user || Date.now() / 1000 > expires) {
    // Try refresh before redirecting
    return <SessionRefresher redirectTo={location.pathname} />;
  }
  
  if (requiredRole && user.role !== requiredRole) {
    return <Navigate to="/dashboard" replace />;
  }
  
  return <>{children}</>;
}
```

### 7.3 Session Refresher Component

```typescript
// SessionRefresher.tsx
function SessionRefresher({ redirectTo }: { redirectTo: string }) {
  const [state, setState] = useState<'loading' | 'error'>('loading');
  
  useEffect(() => {
    let cancelled = false;
    
    async function tryRefresh() {
      try {
        const res = await fetch('/api/auth/refresh', {
          method: 'POST',
          credentials: 'include', // Send refresh_token cookie
        });
        
        if (!res.ok) throw new Error('Refresh failed');
        
        const data = await res.json();
        
        if (!cancelled) {
          localStorage.setItem('auth_token', data.auth_token);
          localStorage.setItem('auth_token_expires', String(data.expires_at));
          localStorage.setItem('auth_user', JSON.stringify(data.user));
          window.location.href = redirectTo;
        }
      } catch {
        if (!cancelled) {
          setState('error');
          // IMPORTANT: Do NOT clear localStorage — preserve state
          // Only redirect to signin, user can come back
          setTimeout(() => {
            window.location.href = `/signin?redirect=${encodeURIComponent(redirectTo)}`;
          }, 2000);
        }
      }
    }
    
    tryRefresh();
    return () => { cancelled = true; };
  }, [redirectTo]);
  
  if (state === 'error') {
    return <div>Session expired. Redirecting to sign in...</div>;
  }
  
  return <div className="flex items-center justify-center h-screen">
    <Spinner />
  </div>;
}
```

### 7.4 API Client with Auto-Refresh

```typescript
// api/client.ts
class ApiClient {
  private baseUrl: string;
  
  constructor(baseUrl: string = '') {
    this.baseUrl = baseUrl;
  }
  
  async fetch(path: string, options: RequestInit = {}): Promise<Response> {
    let token = localStorage.getItem('auth_token');
    const expires = Number(localStorage.getItem('auth_token_expires'));
    
    // Refresh if token expires within 5 minutes
    if (!token || Date.now() / 1000 + 300 > expires) {
      const refreshed = await this.tryRefresh();
      if (refreshed) {
        token = localStorage.getItem('auth_token');
      }
    }
    
    const res = await fetch(`${this.baseUrl}${path}`, {
      ...options,
      headers: {
        ...options.headers,
        'Authorization': token ? `Bearer ${token}` : '',
        'Content-Type': 'application/json',
        'X-Requested-With': 'XMLHttpRequest',
      },
      credentials: 'include', // Always send cookies
    });
    
    // If still 401, try refresh one more time
    if (res.status === 401) {
      const refreshed = await this.tryRefresh();
      if (refreshed) {
        return this.fetch(path, options); // Retry with new token
      }
    }
    
    return res;
  }
  
  private async tryRefresh(): Promise<boolean> {
    try {
      const res = await fetch('/api/auth/refresh', {
        method: 'POST',
        credentials: 'include',
      });
      
      if (!res.ok) return false;
      
      const data = await res.json();
      localStorage.setItem('auth_token', data.auth_token);
      localStorage.setItem('auth_token_expires', String(data.expires_at));
      localStorage.setItem('auth_user', JSON.stringify(data.user));
      return true;
    } catch {
      return false;
    }
  }
}

export const api = new ApiClient();
```

---

## 8. CLI — `nx` Client Tool

### 8.1 Design

Built with **Cobra** (Go). Single binary. Cross-platform. No dependencies.

```
nx
├── nx push        Push binary to sandbox
├── nx promote     Promote preview → production
├── nx list        List sandboxes
├── nx logs        Stream logs from sandbox
├── nx db backup   Backup database
├── nx db restore  Restore database
├── nx db download Download backup file
├── nx env         Manage environment variables
├── nx domain      Manage custom domains
├── nx terminal    Open terminal (SFTP/SSH) to sandbox
├── nx config      Configure CLI (API key, host URL)
├── nx version     Show version info
└── nx help        Help
```

### 8.2 Key Commands

```bash
# Push a binary
nx push ./myapp
# → Uploads binary via SFTP
# → Creates/updates sandbox
# → Returns preview URL

# Promote to production
nx promote myapp
# → Swaps production traffic to latest preview
# → Zero-downtime (new connections only)

# Backup database
nx db backup myapp
# → Creates pg_dump in sandbox
# → Returns download URL

# Download backup
nx db download myapp ./backups/myapp-2026-07-05.sql
# → Downloads via SFTP
```

### 8.3 Configuration

```yaml
# ~/.nx/config.yaml
host: nxsandbox.com
port: 2222
api_key: nx_key_abc123...
default_app: myapp
```

### 8.4 Auth

CLI uses API keys (not OTP). Generated in dashboard. Stored in `~/.nx/config.yaml`.

```
POST /api/auth/cli
Header: X-NX-API-Key: nx_key_...
→ Returns short-lived JWT for subsequent commands
```

---

## 9. Sandbox Engine — bwrap + Postgres

### 9.1 Sandbox Lifecycle

```
CREATE → UPLOAD → START → RUNNING → STOP → ARCHIVE
               ↓         ↓          ↓
             (retry)   (crash)   (manual)
```

### 9.2 Sandbox Manager (Go)

```go
type Sandbox struct {
    ID          string
    AppID       string
    Port        int
    BinaryPath  string
    DBName      string
    DBUser      string
    DBPassword  string
    Status      SandboxStatus // creating, running, stopped, crashed, archived
    PID         int
    Cmd         *exec.Cmd
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type SandboxManager struct {
    sandboxes map[string]*Sandbox
    mu        sync.RWMutex
    db        *sql.DB
    portAlloc *PortAllocator
}

func (sm *SandboxManager) Start(appID string) error {
    s := sm.sandboxes[appID]
    
    // 1. Allocate port
    port := sm.portAlloc.Allocate()
    
    // 2. Create database if needed
    sm.createDatabase(s.DBName, s.DBUser, s.DBPassword)
    
    // 3. Prepare environment
    env := []string{
        fmt.Sprintf("PORT=%d", port),
        fmt.Sprintf("DATABASE_URL=postgres://%s:%s@localhost:5432/%s", 
            s.DBUser, s.DBPassword, s.DBName),
        fmt.Sprintf("NX_APP_ID=%s", appID),
        "NX_SANDBOX=true",
    }
    
    // 4. Launch bwrap
    cmd := exec.Command("bwrap",
        "--ro-bind", "/usr", "/usr",
        "--ro-bind", "/lib", "/lib",
        "--ro-bind", "/lib64", "/lib64",
        "--ro-bind", "/bin", "/bin",
        "--tmpfs", "/tmp",
        "--proc", "/proc",
        "--dev", "/dev",
        "--bind", filepath.Join("/data/sandboxes", appID), "/app",
        "--unshare-user",
        "--unshare-ipc",
        "--unshare-pid",
        "--unshare-net",
        "--hostname", fmt.Sprintf("sandbox-%s", appID),
        s.BinaryPath,
    )
    cmd.Env = append(os.Environ(), env...)
    cmd.Dir = filepath.Join("/data/sandboxes", appID)
    
    // 5. Start and monitor
    if err := cmd.Start(); err != nil {
        return fmt.Errorf("bwrap start failed: %w", err)
    }
    
    s.PID = cmd.Process.Pid
    s.Port = port
    s.Status = "running"
    s.Cmd = cmd
    
    // 6. Health check
    go sm.monitor(s)
    
    return nil
}
```

### 9.3 Preview URLs & Promotion

```go
type RoutingManager struct {
    proxy *httputil.ReverseProxy
}

// Route: *.nxapp.dev → sandbox routing
// Preview: {app}-{version}.nxapp.dev → sandbox:{preview_port}
// Production: {app}.nxapp.dev → sandbox:{production_port}
// Custom: {custom_domain} → sandbox:{production_port}

func (rm *RoutingManager) Route(host string) (*Sandbox, error) {
    // Parse host, determine if preview or production
    // Return target sandbox
}

func (rm *RoutingManager) Promote(appID, version string) error {
    // 1. Health check preview sandbox
    // 2. Update routing table
    // 3. Graceful: drain connections to old production (5s)
    // 4. Switch production port to preview port
    // 5. Old sandbox stays running (can be stopped manually)
    return nil
}
```

### 9.4 Database Management

```go
func (sm *SandboxManager) CreateDatabase(name, user, password string) error {
    _, err := sm.db.Exec(fmt.Sprintf(
        "CREATE USER %s WITH PASSWORD '%s'",
        pq.QuoteIdentifier(user),
        password,
    ))
    if err != nil { return err }
    
    _, err = sm.db.Exec(fmt.Sprintf(
        "CREATE DATABASE %s OWNER %s",
        pq.QuoteIdentifier(name),
        pq.QuoteIdentifier(user),
    ))
    return err
}

func (sm *SandboxManager) BackupDatabase(name string) (string, error) {
    backupPath := filepath.Join("/data/backups", name, 
        time.Now().Format("2006-01-02-150405")+".sql")
    cmd := exec.Command("pg_dump", 
        "-h", "localhost",
        "-d", name,
        "-f", backupPath,
        "--no-owner",
        "--no-acl",
    )
    if err := cmd.Run(); err != nil {
        return "", err
    }
    return backupPath, nil
}
```

---

## 10. API Design

### 10.1 REST API Endpoints

#### Auth
```
POST   /api/auth/signin              Send OTP to email
POST   /api/auth/verify              Verify OTP → return tokens
POST   /api/auth/refresh             Refresh access token (uses cookie)
POST   /api/auth/signoff             Revoke refresh token, clear cookie
GET    /api/auth/verify?token=&email= Magic link verification
POST   /api/auth/cli                 API key → JWT for CLI
```

#### Apps (Sandboxes)
```
GET    /api/apps                     List user's apps
POST   /api/apps                     Create new app (metadata only)
GET    /api/apps/:id                 Get app details
DELETE /api/apps/:id                 Archive app
PUT    /api/apps/:id/settings        Update app settings
POST   /api/apps/:id/binary          Upload binary (multipart)
GET    /api/apps/:id/deployments     List deployments
POST   /api/apps/:id/deployments/:vid/promote  Promote to production
POST   /api/apps/:id/start           Start sandbox
POST   /api/apps/:id/stop            Stop sandbox
POST   /api/apps/:id/restart         Restart sandbox
GET    /api/apps/:id/logs            Stream logs (SSE)
GET    /api/apps/:id/metrics         CPU/RAM/requests
```

#### Database
```
POST   /api/apps/:id/db/backup       Create backup
GET    /api/apps/:id/db/backups      List backups
GET    /api/apps/:id/db/backups/:bid  Download backup
POST   /api/apps/:id/db/restore      Restore from backup
GET    /api/apps/:id/db/credentials   Show connection string
```

#### Admin (Role: admin)
```
GET    /api/admin/users              List all users
POST   /api/admin/users/:uid/role    Change user role
GET    /api/admin/host/metrics       Host CPU/RAM/disk
GET    /api/admin/host/sandboxes     All sandboxes on host
GET    /api/admin/audit              Audit logs
POST   /api/admin/maintenance        Toggle maintenance mode
```

### 10.2 Response Format

```json
{
  "success": true,
  "data": { ... },
  "error": null,
  "meta": { "page": 1, "total": 42 }
}
```

Error:
```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "AUTH_EXPIRED",
    "message": "Access token expired",
    "recoverable": true
  }
}
```

### 10.3 Rate Limiting

| Endpoint | Limit | Window |
|----------|:-----:|:------:|
| `/api/auth/signin` | 5 | 15 min |
| `/api/auth/verify` | 10 | 15 min |
| `/api/auth/refresh` | 30 | 1 min |
| All other `/api/*` | 300 | 1 min |
| CLI endpoints | 600 | 1 min |

---

## 11. Database Schema

### 11.1 Host Database (`nxsandbox`)

```sql
-- Users (auto-created on first OTP sign-in)
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT NOT NULL UNIQUE,
    role        TEXT NOT NULL DEFAULT 'user',  -- 'user' | 'admin'
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_signin TIMESTAMPTZ
);

-- OTP codes (short-lived)
CREATE TABLE otp_codes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email      TEXT NOT NULL,
    hash       TEXT NOT NULL,  -- bcrypt hash of OTP
    expires_at TIMESTAMPTZ NOT NULL,
    used       BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_otp_email_expires ON otp_codes(email, expires_at);

-- Refresh tokens
CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    hash       TEXT NOT NULL,  -- SHA-256 hash of token
    expires_at TIMESTAMPTZ NOT NULL,
    revoked    BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_refresh_user ON refresh_tokens(user_id);

-- API keys (for CLI)
CREATE TABLE api_keys (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash   TEXT NOT NULL,  -- SHA-256 of API key
    name       TEXT NOT NULL,  -- User-given name (e.g., "laptop")
    last_used  TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked    BOOLEAN NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX idx_apikey_hash ON api_keys(key_hash);

-- Apps / Sandboxes
CREATE TABLE apps (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL UNIQUE,
    status          TEXT NOT NULL DEFAULT 'created',
    port            INTEGER,
    binary_path     TEXT,
    binary_version  TEXT,
    binary_checksum TEXT,  -- SHA-256
    db_name         TEXT,
    db_user         TEXT,
    db_password_enc TEXT,  -- Encrypted at rest
    env_vars        JSONB DEFAULT '{}',
    custom_domain   TEXT,
    production_vid  UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_apps_slug ON apps(slug);
CREATE INDEX idx_apps_user ON apps(user_id);

-- Deployments (version history)
CREATE TABLE deployments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id          UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    version         TEXT NOT NULL,
    binary_path     TEXT NOT NULL,
    binary_size     BIGINT,
    checksum        TEXT NOT NULL,  -- SHA-256
    status          TEXT NOT NULL DEFAULT 'uploaded',
    port            INTEGER,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_deployments_app ON deployments(app_id);

-- Database backups
CREATE TABLE backups (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id     UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    file_path  TEXT NOT NULL,
    file_size  BIGINT,
    checksum   TEXT,
    status     TEXT NOT NULL DEFAULT 'created',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_backups_app ON backups(app_id);

-- Audit logs
CREATE TABLE audit_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users(id),
    action      TEXT NOT NULL,
    resource    TEXT NOT NULL,
    resource_id UUID,
    details     JSONB,
    ip_address  INET,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_user ON audit_logs(user_id);
CREATE INDEX idx_audit_created ON audit_logs(created_at);

-- Sessions
CREATE TABLE sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_agent TEXT,
    ip_address INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sessions_user ON sessions(user_id);
```

---

## 12. Styling & Design System

### 12.1 TailwindCSS — Local Build, Inline Classes Only

```bash
# Build Tailwind for SSR pages
npx @tailwindcss/cli -i ./styles/ssr-input.css -o ./static/css/ssr.css --minify

# Build Tailwind for React dashboard
# Vite handles this automatically with @tailwindcss/vite
```

**Critical rule:** No `@apply` in CSS files. No component classes. **Inline utility classes only.**

```html
<!-- ✅ CORRECT: Inline classes -->
<button class="inline-flex items-center justify-center rounded-lg bg-indigo-600 
  px-4 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500 
  transition-colors">
  Deploy
</button>

<!-- ❌ WRONG: Custom CSS component class -->
<button class="btn-primary">Deploy</button>
```

### 12.2 Theme

**Default: Dark.** Optional: Light, System.

```html
<html class="dark" data-theme="dark">
  <!-- dark: default -->
  <!-- data-theme="light": user selected light -->
  <!-- data-theme="system": follow OS -->
</html>
```

### 12.3 Color Palette

```
Primary:    Indigo 600 (#4F46E5)
  - hover:  Indigo 500 (#6366F1)
  - muted:  Indigo 900 (#312E81) 

Accent:     Cyan 400 (#22D3EE) — used sparingly for CTAs

Background (dark):
  - base:   Gray 950 (#030712)
  - surface: Gray 900 (#111827)
  - raised:  Gray 800 (#1F2937)
  - border:  Gray 700 (#374151)

Text (dark):
  - primary:   Gray 100 (#F3F4F6)
  - secondary: Gray 400 (#9CA3AF)
  - muted:     Gray 500 (#6B7280)

Success:  Emerald 500
Warning:  Amber 500
Error:    Red 500
Info:     Blue 500
```

### 12.4 Typography

```
Font: Inter (sans-serif)
Mono: JetBrains Mono (code blocks, terminal)

Scale:
  xs: 0.75rem | sm: 0.875rem | base: 1rem | lg: 1.125rem
  xl: 1.25rem | 2xl: 1.5rem | 3xl: 1.875rem | 4xl: 2.25rem | 5xl: 3rem
```

### 12.5 Component Design Tokens

```
Button:
  rounded-lg (8px), px-4 py-2.5 (default), px-6 py-3 (large)
  font-semibold text-sm

Input:
  rounded-lg border-gray-700 bg-gray-900, px-3 py-2 text-sm
  focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500

Card:
  rounded-xl bg-gray-900 border border-gray-800, p-6 (default)

Badge:
  rounded-full px-2.5 py-0.5 text-xs font-medium

Modal:
  rounded-2xl bg-gray-900 border border-gray-700
  backdrop: bg-black/50 backdrop-blur-sm
```

---

## 13. SEO Strategy

### 13.1 SSR-First for Public Pages

All public pages (`/`, `/signin`, `/docs`, `/pricing`) are server-rendered in Go. No client-side hydration needed:
- Full HTML delivered on first request
- Search engines see complete content
- No JavaScript dependency for indexable content
- Fast LCP (Largest Contentful Paint) — sub-500ms

### 13.2 Meta Tags (per-page, Go template helper)

```go
func MetaTags(title, description, canonical string) template.HTML {
    return template.HTML(fmt.Sprintf(`
<title>%s — NX Sandbox</title>
<meta name="description" content="%s" />
<link rel="canonical" href="https://nxsandbox.com%s" />
<meta property="og:title" content="%s — NX Sandbox" />
<meta property="og:description" content="%s" />
<meta property="og:image" content="https://nxsandbox.com/static/og-default.png" />
<meta name="twitter:card" content="summary_large_image" />
    `, title, description, canonical, title, description))
}
```

### 13.3 Structured Data

```html
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  "name": "NX Sandbox",
  "applicationCategory": "DeveloperApplication",
  "operatingSystem": "Linux",
  "offers": { "@type": "Offer", "price": "0", "priceCurrency": "USD" },
  "description": "Open-source PaaS — push pre-built binaries, colocated Postgres, bwrap isolation."
}
</script>
```

### 13.4 Technical SEO Headers

```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: strict-origin-when-cross-origin
```

---

## 14. Deployment & Packaging

### 14.1 Single Binary Build

```bash
make build

# Under the hood:
# 1. Build React dashboard: cd dashboard && bun run build
# 2. Build Tailwind SSR styles: npx @tailwindcss/cli --minify -o static/css/ssr.css
# 3. Compile Go binary with embeds:
#    CGO_ENABLED=0 go build -ldflags="-s -w" -o nxsandbox ./cmd/nxsandbox

# Result: ~15 MB single binary
```

### 14.2 Makefile

```makefile
.PHONY: build dev test clean

build: build-dashboard build-css build-go

build-dashboard:
	cd dashboard && bun install && bun run build

build-css:
	npx @tailwindcss/cli -i ./styles/ssr-input.css -o ./static/css/ssr.css --minify

build-go:
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=$(shell git describe --tags --always)" -o nxsandbox ./cmd/nxsandbox

dev:
	go run ./cmd/nxsandbox --dev

test:
	go test ./... -race -cover

clean:
	rm -f nxsandbox
	rm -rf dashboard/dist
	rm -f static/css/ssr.css
```

### 14.3 Directory Structure

```
nxsandbox/
├── cmd/
│   └── nxsandbox/
│       └── main.go              # Entry point
├── internal/
│   ├── server/
│   │   ├── server.go            # HTTP server setup
│   │   ├── routes.go            # Route registration
│   │   └── middleware.go        # Auth, logging, rate limit
│   ├── auth/
│   │   ├── auth.go              # OTP, JWT, refresh logic
│   │   ├── otp.go               # OTP generation & verification
│   │   └── middleware.go        # JWT validation middleware
│   ├── sandbox/
│   │   ├── manager.go           # Sandbox lifecycle
│   │   ├── bwrap.go             # bwrap invocations
│   │   └── routing.go           # Preview/production routing
│   ├── database/
│   │   ├── postgres.go          # Connection pool, migrations
│   │   └── migrations/          # Embedded SQL migrations
│   ├── api/
│   │   ├── apps.go              # /api/apps handlers
│   │   ├── auth.go              # /api/auth handlers
│   │   ├── admin.go             # /api/admin handlers
│   │   └── response.go          # JSON response helpers
│   ├── sftp/
│   │   └── sftp.go              # SFTP server
│   ├── email/
│   │   └── email.go             # OTP email (Resend/SES)
│   └── config/
│       └── config.go            # Configuration
├── templates/
│   ├── pages/                   # home.html, signin.html, etc.
│   ├── layouts/base.html        # Base layout
│   └── partials/                # header.html, footer.html
├── static/
│   ├── css/ssr.css              # Built Tailwind for SSR
│   ├── js/ssr.js                # Minimal progressive enhancement
│   ├── fonts/
│   └── favicon.ico, robots.txt, og-default.png
├── dashboard/
│   ├── src/
│   │   ├── main.tsx, App.tsx
│   │   ├── api/client.ts        # API client with auto-refresh
│   │   ├── components/          # ProtectedRoute, SessionRefresher, Layout
│   │   └── pages/               # DashboardHome, AppsList, AppDetail, admin/*
│   ├── index.html, package.json, tsconfig.json, vite.config.ts
│   └── tailwind.config.ts
├── cli/
│   ├── cmd/                     # root.go, push.go, promote.go, db.go, ...
│   └── main.go
├── styles/
│   └── ssr-input.css            # Tailwind input for SSR pages
├── scripts/
│   ├── install.sh               # One-liner install
│   └── docker-compose.yml       # Dev environment
├── docs/                        # index.md, quickstart.md, cli.md, self-hosting.md
├── Makefile, Dockerfile
├── go.mod, go.sum
├── LICENSE                       # MIT
├── README.md, CHANGELOG.md, CONTRIBUTING.md, CODE_OF_CONDUCT.md, SECURITY.md
└── nx-sandbox-PRD.md            # This document
```

### 14.4 go.mod

```
module github.com/digimon99/nxsandbox

go 1.24

require (
    github.com/go-chi/chi/v5       // Router
    github.com/golang-jwt/jwt/v5    // JWT
    github.com/jackc/pgx/v5         // PostgreSQL driver
    github.com/pkg/sftp             // SFTP server
    golang.org/x/crypto             // bcrypt, ssh
    github.com/spf13/cobra          // CLI framework
    github.com/resend/resend-go     // Email (OTP)
)
```

---

## 15. Open Source Strategy

### 15.1 License

**MIT License.** No copyleft. No AGPL. Maximum adoption.

**Rationale:** MIT is the license of Kubernetes, React, Caddy, and every infrastructure tool that won. Coolify (55K stars) is MIT. AGPL scares away commercial users who'd become paying cloud customers. For NX Sandbox — where the cloud-hosted version IS the business model — MIT self-hosted is marketing, not competition.

### 15.2 Open-Core Model

```
MIT (Self-Hosted)              → All features, self-hosted, free forever
Cloud (nxsandbox.com)          → Managed, $0 + usage tiers
Enterprise (Future)            → SSO, SLA, dedicated hosts, gVisor tier
```

### 15.3 Community Building

- **GitHub Discussions** — Primary community hub
- **Discord** — Real-time support
- **Contribution ladder** — Good first issue → docs → features → maintainer
- **RFC process** — Major features proposed and discussed publicly
- **Swag** — Stickers for first 1,000 contributors

### 15.4 GitHub Repository

```
https://github.com/digimon99/nxsandbox

Repository setup:
├── .github/
│   ├── workflows/              # ci.yml (Go tests + lint), release.yml
│   ├── ISSUE_TEMPLATE/
│   ├── PULL_REQUEST_TEMPLATE.md
│   └── CODEOWNERS
```

---

## 16. Milestones & Roadmap

### Phase 1: Core (Weeks 1-8)

- [ ] Go project scaffold with Chi router
- [ ] OTP auth (signin → verify → JWT + refresh token)
- [ ] SSR pages: `/`, `/signin`, `/signup`, `/signin/verify`, `/verified`, `/signoff`
- [ ] React dashboard scaffold with auth guard
- [ ] API: `/api/auth/*` endpoints
- [ ] PostgreSQL schema + migrations
- [ ] Single binary build pipeline
- [ ] bwrap sandbox manager (create, start, stop)
- [ ] SFTP server for binary uploads
- [ ] `nx push`, `nx list`, `nx promote` CLI

### Phase 2: Dashboard (Weeks 9-12)

- [ ] Dashboard home (metrics, app list)
- [ ] App detail page (logs, metrics, preview URL)
- [ ] App settings (env vars, domain)
- [ ] Database management UI (backup, restore, download)
- [ ] Deployment history
- [ ] Admin pages (users, host, audit)
- [ ] `nx logs`, `nx db`, `nx env`, `nx domain` CLI

### Phase 3: Production (Weeks 13-16)

- [ ] Health checks with auto-restart
- [ ] Log aggregation (structured logging)
- [ ] Metrics (Prometheus endpoint)
- [ ] Cloudflare DNS integration
- [ ] Automated backup scheduling
- [ ] Rate limiting
- [ ] Security audit (bwrap profile review)
- [ ] Documentation site
- [ ] One-liner install script
- [ ] Docker Compose for quick self-hosting

### Phase 4: Growth (Months 5-6)

- [ ] Custom domain support
- [ ] GitHub Actions integration (nx-push-action)
- [ ] Team/organization support
- [ ] Usage-based billing (cloud version)
- [ ] gVisor sandbox tier (multi-tenant)
- [ ] Terraform provider
- [ ] Official SDKs (TypeScript, Python)

### Phase 5: Scale (Months 7-12)

- [ ] Multi-host orchestration
- [ ] Geographic region support
- [ ] Automatic horizontal scaling
- [ ] SOC 2 compliance (cloud version)
- [ ] Enterprise SSO (SAML/OIDC)
- [ ] Managed database backups (S3-compatible)

---

## 17. Appendix — Competitive Context

### 17.1 Full Competitive Matrix

| | NX Sandbox | Vercel | Railway | Fly.io | Coolify | Northflank |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| **Build Model** | Pre-built binary | Git push → build | Git push → build | Docker build | Git push → build | OCI Image |
| **Isolation** | bwrap | Firecracker | Docker | Firecracker | Docker | Kata + gVisor |
| **Database** | Colocated Postgres | 3rd-party | Shared Postgres | Remote | Shared Postgres | Add-on |
| **App→DB Latency** | <1ms | 10-50ms | 1-5ms | 1-10ms | 1-5ms | 1-10ms |
| **Lock-in** | Zero | High | Medium | Medium | Low | Low |
| **License** | MIT | Proprietary | Proprietary | Proprietary | MIT | Proprietary |
| **Self-hosted** | ✅ | ❌ | ❌ | ❌ | ✅ | BYOC |
| **Cold start** | <100ms | 200-500ms | 1-3s | 300ms-3s | 1-5s | 1-5s |
| **GitHub Stars** | New | N/A | N/A | N/A | 55K | N/A |

### 17.2 Empty Quadrant

```
                     Build on Deploy          Pre-Built Binary
                     ──────────────           ────────────────
Managed Platform     Vercel, Railway           ★ NX Sandbox ★
                     Fly.io, Render            (no one else)

Self-Hosted          Coolify, Dokploy          scp + systemd
                     CapRover                  (manual)
```

**NX Sandbox is alone in the top-right quadrant.** No managed platform offers no-build deployment.

### 17.3 bwrap Security Note

Every security analysis reviewed confirms: bwrap shares the host kernel. A kernel CVE → escape. This was demonstrated in the wild with Claude Code's bwrap sandbox (CVE-2026-25725). 

**Mitigation for NX Sandbox:**
- Single-tenant model at launch — users run their own code in their own sandbox
- Regular kernel updates (Ubuntu livepatch)
- Seccomp profiles as defense-in-depth
- gVisor tier for multi-tenant (Phase 4)
- This is the #1 technical risk — document it clearly, don't hide it

---

*This PRD is a living document. Last updated: 2026-07-05. Next review: after Phase 1 completion.*