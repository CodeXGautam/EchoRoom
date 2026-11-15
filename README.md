# EchoRoom

**A cosy room where little doodles wander around and chat.**

Pick a name and a colour, walk your doodle around a shared room, and whatever you type pops up as a speech bubble above your head. Everyone online in the room can see you, and you can sit at tables or keep permanent group chats.

Under the hood it is a real-time distributed system: a Go WebSocket service with one goroutine per room, Redis pub/sub for fan-out across replicas, Supabase for Postgres and auth, and Kubernetes manifests for running several replicas locally.

> Status: **Phase 1** done: guests join one lobby and see each other's doodles move in real time. Chat bubbles are next. See [docs/PLAN.md](docs/PLAN.md).

---

## Architecture

```text
 Browser ── Next.js + Phaser (Vercel)
   │  HTTPS  POST /api/v1/ws/ticket   (Authorization: Bearer <Supabase JWT>)
   │  WSS    /ws?ticket=…             (one-time ticket, 30 s)
   ▼
 Go API (Render · Docker · kind/k8s locally)
   ├─ auth     verify Supabase JWT via JWKS
   ├─ ws       ticket store, read/write pumps, rate limit, pings
   └─ room     one goroutine per room: join/leave/move, 10 Hz batched broadcast
   │                                  │
   ▼                                  ▼
 Supabase Postgres + Auth          Redis / Valkey (tickets · pub/sub · presence)
```

**Key design choices**

- **Actor-style rooms.** Each room's state is owned by a single goroutine, and everything else talks to it over channels. There are no mutexes and no data races by construction.
- **Batched movement.** Moves are collected and broadcast once per 100 ms tick, which gives N frames per tick instead of N².
- **Client-side prediction, server validation.** Your doodle moves instantly. The server rejects moves faster than `MaxSpeed` and sends a `correct` frame back.
- **Slow clients are dropped, not waited on.** Each client has a bounded send buffer, so one bad connection cannot stall a room.
- **WebSocket tickets.** Browsers can't send auth headers on WebSockets, so the JWT is exchanged for a single-use ticket (Redis `GETDEL`) instead of being put in the URL.
- **Graceful shutdown.** On SIGTERM the server drains HTTP, then closes every socket with code 1001 so clients reconnect (to another pod, in Kubernetes).

## Tech stack

| Layer | Tech |
|---|---|
| Frontend | Next.js 16, React 19, TypeScript, Tailwind CSS 4, Phaser 4, Zustand, supabase-js |
| Backend | Go 1.25, chi, coder/websocket, pgx, go-redis, golang-jwt + keyfunc, slog |
| Data | Supabase Postgres, Supabase Auth, Redis 7 / Valkey 8 |
| Infra | Docker, docker-compose, Render, Vercel, Kubernetes (kind) *(Phase 3+)* |

## Project structure

```text
backend/
  cmd/api/            entry point, wiring, graceful shutdown
  internal/auth/      Supabase JWT + dev-token verification, HTTP middleware
  internal/room/      room actor, protocol, client, manager   ← the core
  internal/ws/        tickets, WebSocket handler (read/write pumps)
  internal/http/      router, health/readiness
  internal/config/    env config (PORT, DATABASE_URL, REDIS_URL, …)
  internal/db, redis, middleware, logger
frontend/
  app/                landing page, /room
  game/               Phaser scene + Doodle
  lib/                api, auth, protocol types, RoomSocket (reconnect)
  store/              persisted session (name, colour)
docs/PLAN.md          roadmap and decisions
render.yaml           Render blueprint (API + Key Value)
docker-compose.yml    local Postgres + Redis (+ API)
```

## Run locally

Prerequisites: Go 1.25+, Node 22+, pnpm, Docker.

```bash
# 1. Dependencies (use other host ports if you already run Postgres/Redis natively)
docker compose up -d postgres redis
#   POSTGRES_HOST_PORT=55432 REDIS_HOST_PORT=56379 docker compose up -d postgres redis

# 2. API — dev mode needs no Supabase project
cd backend
cp .env.example .env        # adjust DATABASE_URL / REDIS_URL ports if you changed them
go run ./cmd/api            # http://localhost:8080/healthz

# 3. Frontend
cd ../frontend
cp .env.example .env.local
pnpm install
pnpm dev                    # http://localhost:3000
```

Open two browser tabs. In dev mode each tab is a separate guest, so you'll see both doodles.

Tests (race detector on):

```bash
cd backend && go test -race ./...
```

## Deploy (free tier)

**1. Supabase**
- Create a project in the Singapore region (`ap-southeast-1`).
- Authentication → Sign In / Providers → enable **Anonymous sign-ins**. Turning on CAPTCHA is recommended, to stop bots creating anonymous users.
- Project Settings → JWT Keys: make sure the project uses **JWT signing keys**. For a legacy HS256 project, migrate it, or set `SUPABASE_JWT_SECRET` on the API.
- Connect → **Session pooler** connection string: this is `DATABASE_URL`.

**2. Render**
- New → Blueprint → select this repo. That creates `echoroom-api` and `echoroom-kv` from `render.yaml`.
- Fill in `SUPABASE_URL`, `DATABASE_URL`, and `ALLOWED_ORIGINS` (your Vercel URL).

**3. Vercel**
- Import the repo with Root Directory `frontend`.
- Set these environment variables:
  - `NEXT_PUBLIC_API_URL`: your Render URL
  - `NEXT_PUBLIC_AUTH_MODE`: `supabase`
  - `NEXT_PUBLIC_SUPABASE_URL`
  - `NEXT_PUBLIC_SUPABASE_ANON_KEY`

The free Render service sleeps after 15 minutes idle and takes about a minute to wake. The room page shows a "waking up" message while that happens.

## Protocol

Every WebSocket frame is `{"t": "<type>", "d": {...}}`.

| Direction | Type | Payload |
|---|---|---|
| → server | `move` | `x, y, dir, moving` |
| ← client | `welcome` | `selfId, world{w,h}, players[]` |
| ← client | `joined` / `left` | `Player` / `{id}` |
| ← client | `state` | `players[]` (batched, 10 Hz) |
| ← client | `correct` | `x, y` (your move was rejected) |

Close codes: `1001` server restarting (reconnect) · `4001` opened in another tab (don't reconnect) · `4002` too slow · `4003` flooding.

## Roadmap

See [docs/PLAN.md](docs/PLAN.md). Next up: quick rooms with invite codes and chat bubbles (Phase 2), then Redis fan-out and your first local Kubernetes cluster (Phase 3).

## Developer

**Himanshu Sharma**
