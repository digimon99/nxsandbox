# NX Sandbox

Push binary and serve instantly.

NX Sandbox is an open-source PaaS for deploying pre-built binaries into lightweight Linux sandboxes with colocated PostgreSQL.

It is designed for teams that want fast deployments without remote image builds or heavy platform lock-in.

Status: Early stage, actively evolving
License: MIT

Suggested GitHub About description:

Open-source binary-first PaaS. Push pre-built binaries and serve instantly with lightweight sandbox isolation and colocated PostgreSQL.

## Table of Contents

1. Overview
2. Key Features
3. Architecture
4. Project Status
5. Quick Start
6. Configuration
7. Build and Release
8. API Surface
9. Repository Layout
10. Roadmap
11. Contributing
12. Security
13. Documentation

## Overview

NX Sandbox focuses on a simple deployment model:

- Build your app binary locally or in CI
- Push the binary to NX Sandbox
- Run it in an isolated runtime with PostgreSQL access

The core design goal is to reduce platform lock-in while keeping deployment UX lightweight.

## Key Features

- Go-based API server with Chi router
- OTP + JWT authentication model
- Refresh-token session flow for resilient auth
- Embedded SQL migrations for startup-safe schema management
- SSR public auth pages served by Go templates
- Local Tailwind CSS pipeline for SSR templates (no CDN dependency)
- Cross-platform build outputs for Windows and Linux amd64

## Architecture

Current baseline modules:

- API and middleware: internal/api
- Authentication and OTP flows: internal/auth
- App and deployment metadata store: internal/apps
- Database bootstrap and migrations: internal/database
- Web SSR handler and templates: internal/web
- Sandbox manager skeleton: internal/sandbox
- Host entrypoint: cmd/nxsandbox
- CLI scaffold: cli

## Project Status

Current implementation is a functional foundation intended for iterative expansion.
Core app runtime orchestration, full dashboard SPA, and deployment execution flow are in progress.

## Quick Start

Prerequisites:

- Go 1.25+
- Node.js and npm (for Tailwind build and optional TypeScript checks)
- PostgreSQL

Setup:

1. Copy environment template values into .env
2. Configure at minimum:
   - POSTGRES_DSN
   - AUTH_JWT_SECRET
   - RESEND_API_KEY and RESEND_FROM for OTP email delivery

Run locally:

    go run ./cmd/nxsandbox

Health endpoint:

    GET http://localhost:8080/health

## Configuration

Main environment variables:

- HOST
- PORT
- POSTGRES_DSN
- AUTH_JWT_SECRET
- AUTH_ACCESS_TOKEN_MINUTES
- AUTH_REFRESH_TOKEN_DAYS
- AUTH_OTP_WINDOW_MINUTES
- ADMIN_EMAILS
- RESEND_API_KEY
- RESEND_FROM

Use .env.example as the baseline reference.

## Build and Release

Primary script:

- build-and-deploy.ps1

What it does:

- Optional TypeScript syntax checks when TS projects are present
- Local Tailwind build to internal/web/static/css/ssr.css
- Style policy checks for SSR templates
- Go module tidy and tests
- CGO disabled cross-builds:
  - nxsandbox.exe
  - nxsandbox-linux-new

Run:

    .\build-and-deploy.ps1

## API Surface

Auth endpoints:

- POST /api/auth/signin
- POST /api/auth/verify
- POST /api/auth/refresh
- POST /api/auth/signoff
- GET /api/auth/me

App endpoints:

- GET /api/apps
- POST /api/apps
- GET /api/apps/{id}/deployments

## Repository Layout

Top-level highlights:

- cmd/nxsandbox
- internal
- cli
- styles
- build-and-deploy.ps1
- .env.example
- .gitignore
- LICENSE

## Roadmap

Near-term priorities:

- Full binary upload and deployment lifecycle
- Runtime health and process orchestration
- Dashboard SPA implementation
- Domain routing and promotion workflow
- Enhanced observability and audit trails

## Contributing

Contributions are welcome.

Recommended workflow:

1. Fork repository
2. Create feature branch
3. Run build-and-deploy.ps1 locally
4. Open pull request with clear scope and test notes

## Security

- Do not commit secrets or production credentials
- Keep .env local and use .env.example for shared defaults
- Report vulnerabilities privately via SECURITY.md once published

## Documentation

Repository hygiene is defined in:

- .gitignore

Note:

- Product requirement documents are kept local and intentionally excluded from GitHub tracking by repository policy.
