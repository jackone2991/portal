# Portal — Sơ đồ hệ thống

Bản đồ kiến trúc trực quan. Sơ đồ dùng Mermaid — render native trong GitHub, GitLab, VS Code preview, và `mermaid.live`. Source là text, nên diff được và version-controlled (không như export Miro/Figma).

Bảy view, mỗi cái trả lời một câu hỏi khác nhau:

1. **Landscape hệ thống** — service nào chạy và data flow giữa chúng thế nào.
2. **Bản đồ module backend** — phân chia modular monolith.
3. **Quy tắc boundary module** — cái gì được import cái gì.
4. **Flow request đã authenticate** — chain middleware trên mọi endpoint protected.
5. **Sequence OIDC login** — handshake auth với Authentik.
6. **Flow upload + transcode asset** — pipeline media end-to-end.
7. **Phase roadmap** — thứ tự implementation.

Sơ đồ giữ nguyên label tiếng Anh (technical terms). Narrative xung quanh là tiếng Việt.

---

## 1. Landscape hệ thống

View "Miro" — mọi component và mọi connection trong một lượt nhìn.

```mermaid
graph TB
    classDef user fill:#e1f5ff,stroke:#0277bd,color:#000
    classDef edge fill:#fff3e0,stroke:#e65100,color:#000
    classDef frontend fill:#f3e5f5,stroke:#6a1b9a,color:#000
    classDef backend fill:#e8f5e9,stroke:#2e7d32,color:#000
    classDef datastore fill:#fce4ec,stroke:#c2185b,color:#000
    classDef external fill:#fafafa,stroke:#616161,color:#000
    classDef observability fill:#f9fbe7,stroke:#827717,color:#000

    Browser[Web Browser]:::user
    PWA[Installable PWA<br/>Web Push]:::user

    subgraph EDGE[Edge / CDN layer]
        direction TB
        R2[Cloudflare R2<br/>HLS chunks edge]:::edge
        Traefik[Traefik v3<br/>reverse proxy + TLS<br/>routes /api -> Go, / -> Next.js]:::edge
    end

    subgraph FRONTEND[Frontend - Next.js 15]
        direction TB
        Next[App Router + RSC<br/>RSC-first; client islands]:::frontend
        APIServer[api-server.ts<br/>server-only fetch wrapper<br/>cookies forwarding]:::frontend
    end

    subgraph BACKENDPROC[Backend processes]
        direction TB
        CmdAPI[cmd/api<br/>Chi HTTP server<br/>port 8080]:::backend
        CmdWorker[cmd/worker<br/>Asynq consumer<br/>transcode/thumbnail/notify]:::backend
        CmdSysJobs[cmd/sysjobs<br/>BYPASSRLS cross-tenant<br/>nightly batch]:::backend
        MediaMTX[mediamtx sidecar<br/>RTMP -> LL-HLS<br/>profile: live]:::backend
        LiveKit[LiveKit SFU<br/>group calls<br/>profile: calls]:::backend
    end

    subgraph DATASTORES[Datastores]
        direction LR
        PG[(Postgres 17<br/>+ PgBouncer<br/>+ RLS policies)]:::datastore
        Dragonfly[(Dragonfly<br/>Redis-compat<br/>cache + Asynq broker + pub/sub)]:::datastore
        MinIO[(MinIO<br/>S3 origin<br/>tenant-prefixed keys)]:::datastore
    end

    subgraph EXT[External services]
        direction TB
        Authentik[Authentik<br/>OIDC IdP<br/>+ TOTP/WebAuthn MFA]:::external
        SMTP[SMTP provider<br/>SES/Mailgun/Postfix]:::external
        WebPush[Web Push<br/>VAPID]:::external
        Stripe[Stripe Connect<br/>creator payouts<br/>opt-in]:::external
    end

    subgraph OBS[Observability profile: opt-in]
        direction LR
        Grafana[Grafana<br/>unified UI]:::observability
        Loki[Loki<br/>logs]:::observability
        Prom[Prometheus<br/>metrics]:::observability
        Tempo[Tempo<br/>traces]:::observability
        GlitchTip[GlitchTip<br/>errors]:::observability
    end

    Browser --> Traefik
    PWA --> Traefik
    Browser -.HLS playback.-> R2
    R2 -.origin pull.-> MinIO

    Traefik --> Next
    Traefik --> CmdAPI
    Next --> APIServer
    APIServer -->|server-side fetch<br/>cookie forwarded| CmdAPI

    CmdAPI <--> PG
    CmdAPI <--> Dragonfly
    CmdAPI <--> MinIO
    CmdAPI -->|enqueue| Dragonfly
    CmdAPI -->|OIDC| Authentik

    CmdWorker <--> Dragonfly
    CmdWorker <--> PG
    CmdWorker <--> MinIO
    CmdWorker --> SMTP
    CmdWorker --> WebPush

    CmdSysJobs --> PG
    CmdSysJobs --> MinIO

    MediaMTX --> MinIO
    LiveKit --> MinIO
    CmdAPI -.token mint.-> LiveKit

    CmdAPI -.metrics+traces+logs+errors.-> OBS
    CmdWorker -.metrics+traces+logs+errors.-> OBS

    Browser -.optional payouts.-> Stripe
```

**Flow chính được show:**

- **Path playback của user** — browser pull HLS chunk trực tiếp từ Cloudflare R2 (origin-pull từ MinIO khi cache miss). Tránh round-trip qua API.
- **Path API** — mọi request đã authenticate đi Browser → Traefik → API.
- **RSC fetch** — server component Next.js gọi API qua `api-server.ts` với cookie forward, không bao giờ expose token cho JS browser.
- **Worker** — process độc lập; consume queue Asynq, hit Postgres + MinIO; emit notification qua SMTP + Web Push.
- **Service optional** — LiveKit (call), mediamtx (live streaming), observability stack đều sau flag `--profile` trong docker-compose; self-host single-VM có thể skip.

---

## 2. Bản đồ module backend

Modular monolith. Một family binary Go duy nhất, nhưng source tree split thành các bounded context.

```mermaid
graph TB
    classDef domain fill:#e3f2fd,stroke:#1565c0,color:#000
    classDef bridge fill:#fff8e1,stroke:#f57f17,color:#000
    classDef cross fill:#fce4ec,stroke:#ad1457,color:#000
    classDef plat fill:#e8eaf6,stroke:#283593,color:#000
    classDef cmd fill:#e0f2f1,stroke:#00695c,color:#000

    subgraph CMD[cmd - wiring layer]
        api[api<br/>HTTP server]:::cmd
        worker[worker<br/>Asynq consumer]:::cmd
        sysjobs[sysjobs<br/>BYPASSRLS]:::cmd
    end

    subgraph DOMAIN[Domain modules - internal/modules/]
        account[account<br/>auth + RBAC<br/>OIDC + JWT]:::domain
        tenant[tenant<br/>orgs + memberships<br/>kind: org/household]:::domain
        media[media<br/>assets + transcode<br/>thumbnail workers]:::domain
        movie[movie<br/>catalog + episodes]:::domain
        music[music<br/>tracks + playlists]:::domain
        story[story<br/>chapters + drafts]:::domain
        comic[comic<br/>pages + readers]:::domain
        bank[bank<br/>double-entry ledger<br/>+ investments]:::domain
        notification[notification<br/>in-app + email + push]:::domain
        social[social<br/>posts + follows<br/>+ communities]:::domain
        creator[creator BRIDGE<br/>tips + subs]:::bridge
        marketplace[marketplace BRIDGE<br/>listings + escrow]:::bridge
        safety[safety<br/>NSFW + CSAM]:::cross
    end

    subgraph PLATFORM[Platform - internal/platform/]
        config[config<br/>env loader]:::plat
        db[db<br/>pgx + BeginTenantScope]:::plat
        cache[cache<br/>Redis + TenantKey]:::plat
        storage[storage<br/>S3 + tenant prefix]:::plat
        jobs[jobs<br/>Asynq client]:::plat
        realtime[realtime<br/>SSE + WebSocket<br/>Dragonfly pub/sub]:::plat
        mail[mail<br/>SMTP wrapper]:::plat
        audit[audit<br/>cross-cutting log]:::plat
        observability[observability<br/>OTel + Sentry]:::plat
        mw[middleware<br/>request-id + ratelimit<br/>+ tenant resolver]:::plat
    end

    api -->|MountHTTP| DOMAIN
    worker -->|RegisterTasks| DOMAIN
    sysjobs --> bank
    sysjobs --> tenant

    movie --> media
    music --> media
    story --> media
    comic --> media

    creator --> social
    creator --> bank

    marketplace --> social
    marketplace --> bank

    safety --> media
    safety --> social

    notification -.consumes notify:*.-> DOMAIN

    DOMAIN --> PLATFORM
    api --> mw
    worker --> jobs
```

**Hướng dẫn đọc:**

- **Domain modules** (xanh dương) — bounded context. Nói chuyện với nhau chỉ qua subpackage `api/`.
- **Bridge modules** (vàng) — `creator` và `marketplace` chủ ý span social + bank.
- **Cross-cutting** (hồng) — `safety` consume event từ `media` + `social` để chạy classifier NSFW/CSAM/toxicity.
- **Platform** (chàm) — không có logic nghiệp vụ; hạ tầng cross-cutting.
- **cmd/** (xanh ngọc) — chỉ wiring; construct mỗi module một lần và gọi `MountHTTP` / `RegisterTasks`.

Giao tiếp cross-module: synchronous qua call `<module>api.X(ctx, ...)`, asynchronous qua event Asynq với naming `<emitting-module>:<event>`.

---

## 3. Quy tắc boundary module

Cái gì được import cái gì — enforce bởi `golangci-lint depguard`.

```mermaid
graph TB
    classDef allowed fill:#c8e6c9,stroke:#2e7d32,color:#000
    classDef restricted fill:#ffccbc,stroke:#bf360c,color:#000
    classDef wiring fill:#bbdefb,stroke:#0d47a1,color:#000

    subgraph CMDLAYER[cmd]
        cmdapi[cmd/api]:::wiring
        cmdworker[cmd/worker]:::wiring
        cmdsysjobs[cmd/sysjobs]:::wiring
    end

    subgraph MODX[Module X]
        Xapi[X/api/<br/>public surface]:::allowed
        Xhandler[X/handler/<br/>HTTP handlers]:::restricted
        Xservice[X/service/<br/>business logic]:::restricted
        Xquery[X/query/<br/>sqlc SQL]:::restricted
        Xrepo[X/repository/<br/>sqlc output]:::restricted
        Xmiddleware[X/middleware/]:::restricted
    end

    subgraph MODY[Module Y]
        Yapi[Y/api/<br/>public surface]:::allowed
        Yhandler[Y/handler/<br/>private]:::restricted
        Yservice[Y/service/<br/>private]:::restricted
    end

    subgraph PLAT[Platform]
        platall[platform/*<br/>infrastructure]:::allowed
    end

    subgraph SYSREPO[sysrepository - BYPASSRLS]
        sysrepo[internal/sysrepository<br/>BYPASSRLS pool]:::restricted
    end

    cmdapi -->|may import| MODX
    cmdapi -->|may import| MODY
    cmdworker --> MODX
    cmdsysjobs -->|exclusive| sysrepo
    cmdsysjobs --> Xapi

    MODX -->|may import| PLAT
    MODY -->|may import| PLAT
    Xservice -.cross-module via API only.-> Yapi

    Xservice -.FORBIDDEN.-> Yservice
    Xservice -.FORBIDDEN.-> Yrepo
    Xservice -.FORBIDDEN.-> Yhandler

    MODX -.FORBIDDEN.-> sysrepo
    MODY -.FORBIDDEN.-> sysrepo

    Xservice --> Xquery
    Xservice --> Xrepo
```

**Quy tắc cứng** (enforce bởi depguard, fail CI):

| Caller | Được import | KHÔNG được import |
|---|---|---|
| `cmd/api`, `cmd/worker` | mọi module, `platform/*` | `internal/sysrepository` |
| `cmd/sysjobs` | `internal/sysrepository` (chỉ nơi duy nhất!), package `api/` của module | — |
| `modules/X/service` | internal của module mình + `platform/*` + chỉ `api/` của module khác | `service/`, `handler/`, `repository/`, `query/`, package subdomain của module khác |
| Module bất kỳ | internal mình + `platform/*` + `api/` khác | `internal/sysrepository` |

Quy tắc load-bearing duy nhất: **module nói chuyện với nhau chỉ qua package `api/`. Không bao giờ JOIN across table của nhau.**

---

## 4. Flow request đã authenticate

Chain middleware trên mỗi endpoint protected, với một nhánh error path show.

```mermaid
sequenceDiagram
    autonumber
    actor U as User Browser
    participant T as Traefik
    participant Mw as Middleware chain
    participant H as Handler
    participant S as Service
    participant R as Repository (sqlc)
    participant PG as Postgres
    participant Ca as Dragonfly cache
    participant Au as audit.Logger

    U->>T: HTTPS request<br/>Cookie: portal_access
    T->>Mw: RealIP + RequestID + Recoverer + Timeout(30s) + CORS

    rect rgb(240, 248, 255)
        Note over Mw: RequireAuth
        Mw->>Mw: parse JWT, verify HS256
        Mw->>PG: SELECT token_version, disabled_at FROM users WHERE id=?
        PG-->>Mw: snapshot
        alt JWT bad / user disabled / token_version mismatch
            Mw-->>U: 401 + Problem
        end
    end

    rect rgb(255, 248, 240)
        Note over Mw: RequireTenant
        Mw->>Mw: extract /t/{slug} from path
        Mw->>Ca: cache lookup tenant_id
        alt cache miss
            Mw->>PG: SELECT tenant + membership check
            PG-->>Mw: tenant_id
            Mw->>Ca: cache set
        end
        Mw->>PG: BEGIN tx + SET LOCAL app.tenant_id GUC
    end

    rect rgb(240, 255, 240)
        Note over Mw: RequireACR (step-up, op nhạy cảm)
        Mw->>Mw: check claim acr + auth_time
        alt ACR không đủ
            Mw-->>U: 403 + Problem step_up_required<br/>(frontend redirect sang /auth/login?step_up=mfa)
        end
    end

    rect rgb(255, 240, 245)
        Note over Mw: RequirePermission
        Mw->>Ca: rbac:perms:userID:tenantID:v<token_version>
        alt cache miss
            Mw->>PG: recursive CTE: roles -> ancestors -> permissions
            PG-->>Mw: effective set
            Mw->>Ca: cache set TTL 5 phut
        end
        alt deny
            Mw-->>U: 403 + Problem
        end
    end

    Mw->>H: pass sang handler với ctx<br/>(identity + tenant + tx)
    H->>S: service.Movies.Create(ctx, input)
    S->>R: repo.Movies.Insert(ctx, ...)
    R->>PG: INSERT (RLS auto-filter theo app.tenant_id)
    PG-->>R: row
    R-->>S: result
    S->>Au: audit.Logger.Write(ctx, "movie.created", ...) (non-blocking)
    Au-->>PG: best-effort insert vao audit_log
    S-->>H: response
    H->>PG: COMMIT
    H-->>U: 201 + Location + body
```

Mỗi route protected walk hết năm lớp middleware theo thứ tự. RLS ở database là **đường phòng thủ cuối cùng**: kể cả khi handler quên clause `WHERE tenant_id = ...`, Postgres từ chối trả row.

---

## 5. Sequence OIDC login

Handshake auth đầy đủ từ "user click Sign In" tới "cookie session đã set".

```mermaid
sequenceDiagram
    autonumber
    actor U as User Browser
    participant N as Next.js<br/>(login button)
    participant A as Portal API<br/>cmd/api
    participant Ak as Authentik<br/>(IdP)
    participant PG as Postgres

    U->>N: click "Sign in"
    N->>A: GET /auth/login
    A->>A: generate state + nonce<br/>set cookie portal_oidc<br/>(HttpOnly, TTL 5 phut)
    A-->>U: 302 sang Authentik authorize<br/>+ Set-Cookie portal_oidc

    U->>Ak: GET /application/o/portal/authorize<br/>?state=...&nonce=...&acr_values=...
    Ak->>U: prompt password
    U->>Ak: nhap credentials
    Ak->>U: prompt MFA (TOTP/WebAuthn)<br/>neu acr_values=mfa
    U->>Ak: code 6 so hoac assertion WebAuthn
    Ak-->>U: 302 sang /auth/callback?code=...&state=...

    U->>A: GET /auth/callback?code=...&state=...<br/>Cookie: portal_oidc
    A->>A: verify state match cookie<br/>(check CSRF)
    A->>Ak: POST /token<br/>code + client_secret
    Ak-->>A: { access_token, id_token, refresh_token }
    A->>A: verify chu ky ID token<br/>verify nonce match cookie

    rect rgb(240, 255, 240)
        Note over A,PG: Upsert user + sync role
        A->>PG: UPSERT users (oidc_subject, email, display_name)
        A->>PG: SYNC user_oidc_roles<br/>tu id_token.groups<br/>(theo OIDC_GROUP_ROLE_MAP)
        opt user co permission bank:* VA amr thieu 'mfa'
            A-->>U: 403 + mfa_enrollment_required<br/>(deep-link sang Authentik MFA dashboard)
        end
    end

    A->>A: mint access JWT (5 phut, HS256)<br/>mint refresh token (256-bit, hashed)
    A->>PG: INSERT refresh_tokens<br/>+ audit account.login
    A-->>U: 302 sang /<br/>+ Set-Cookie portal_access (Path=/)<br/>+ Set-Cookie portal_refresh (Path=/auth)<br/>+ Delete portal_oidc

    U->>N: GET / voi cookie moi
    N-->>U: page home rendered<br/>(da authenticated)
```

Hai cookie được set với path khác nhau nên refresh token chỉ travel sang endpoint `/auth/*`. Rotation refresh-token + reuse detection sống trong call `/auth/refresh` sau đó.

---

## 6. Flow upload + transcode asset

Pipeline media end-to-end show một video upload trở thành HLS playback thế nào.

```mermaid
sequenceDiagram
    autonumber
    actor U as User Browser
    participant A as cmd/api
    participant S as platform/storage
    participant M as MinIO origin
    participant Q as Dragonfly<br/>(queue Asynq)
    participant W as cmd/worker<br/>(handler transcode)
    participant F as FFmpeg
    participant Sa as safety worker<br/>(D-38)
    participant R as Cloudflare R2 CDN

    U->>A: POST /api/v1/t/{tenant}/media/uploads<br/>(multipart)
    A->>A: RBAC: media:upload + check quota<br/>(MAX_QUEUED_TRANSCODES_PER_TENANT)
    A->>S: storage.Put(ctx, key)
    S->>M: PUT org/<tid>/assets/source/<id>.mp4
    A->>A: INSERT assets (status=pending)
    A->>Q: Enqueue task transcode<br/>{tenant_id, asset_id, source_key}
    A-->>U: 202 Accepted + asset_id

    Q->>W: deliver task
    W->>W: TenantMiddleware: BeginTenantScope
    W->>A: UPDATE assets SET status=processing
    W->>F: ffmpeg -i source.mp4<br/>-c:v libx264 (hoac NVENC/VAAPI)<br/>-hls_time 6 -hls_playlist_type vod<br/>-master_pl_name index.m3u8<br/>+ rendition 1080/720/480/360
    F->>M: write HLS segment sang<br/>org/<tid>/assets/hls/<id>/

    par Generate thumbnail
        W->>F: ffmpeg generate poster + sprite
        F->>M: write thumbnail
    end

    W->>A: UPDATE assets SET status=ready,<br/>hls_master_url, duration_ms, thumbnail_url
    W->>Q: emit event media:asset_ready<br/>(consume boi movie/music/etc.)

    par Safety scan (Phase 12+)
        Q->>Sa: media:asset_ready consumed
        Sa->>Sa: chay classifier nsfwjs + phash
        alt CSAM hash match
            Sa->>A: UPDATE assets SET status=quarantined
            Sa->>A: page operator qua webhook
        else NSFW > threshold
            Sa->>A: UPDATE assets SET nsfw_flag=true
        end
    end

    Note over M,R: replication lien tuc async sang R2

    U->>R: GET HLS chunk qua signed URL tu API
    R->>M: cache miss origin pull
    M-->>R: segments
    R-->>U: chunk serve tu edge

    alt transcode fail 3 lan
        W->>Q: move sang transcode:dead
        W->>A: UPDATE assets SET status=failed, error_message
        W->>A: emit event media:asset_failed
    end
```

Toàn bộ flow **non-blocking từ perspective của user**: upload trả về ngay 202, transcode chạy background. Failure route sang dead-letter queue cần action operator.

---

## 7. Phase roadmap

Thứ tự implementation với dependency gating.

```mermaid
graph LR
    classDef done fill:#c8e6c9,stroke:#2e7d32,color:#000
    classDef next fill:#bbdefb,stroke:#0d47a1,color:#000
    classDef later fill:#e0e0e0,stroke:#616161,color:#000

    P0[Phase 0<br/>Wiring foundation]:::next
    P1[Phase 1<br/>Tenancy + RLS]:::later
    P2[Phase 2<br/>Media pipeline]:::later
    P3[Phase 3<br/>Vertical Movies]:::later
    P4[Phase 4<br/>Music/Stories/Comics<br/>+ progress + ratings]:::later
    P5pre[Prereq Phase 5<br/>step-up auth + MFA]:::later
    P5[Phase 5<br/>Bank 5a..5i<br/>ledger/debt/investment]:::later
    P6[Phase 6<br/>Notifications<br/>SSE + email + Web Push]:::later
    P7[Phase 7<br/>Social baseline<br/>newsfeed + follow + DM]:::later
    P8[Phase 8<br/>Search<br/>Postgres FTS]:::later
    P9[Phase 9<br/>Marketing microsite<br/>+ blog + badges]:::later
    P10[Phase 10<br/>Social advanced<br/>stories + reels + live]:::later
    P11[Phase 11<br/>Creator economy<br/>tips + subs + payouts]:::later
    P12[Phase 12<br/>Marketplace + ML safety<br/>+ voice/video calls]:::later

    P0 --> P1
    P1 --> P2
    P2 --> P3
    P3 --> P4
    P0 -.prereq.-> P5pre
    P5pre --> P5
    P4 --> P6
    P5 --> P6
    P6 --> P7
    P7 --> P8
    P7 --> P10
    P9 -.parallel.-> P0
    P10 --> P11
    P5 --> P11
    P11 --> P12
    P10 --> P12
```

**Quy tắc gate:**

- Tiêu chí exit của Phase N phải đạt trước khi Phase N+1 mở.
- Phase 5 (bank) gate bởi sub-phase prereq tường minh land step-up auth + MFA enforcement trước — op money không thể ship mà không có chúng.
- Phase 9 (microsite) đủ độc lập để ship parallel với bất kỳ phase nào khác khi Phase 0 xong.
- Phase 10–12 build trên trio social + creator + bank.

---

## Source sơ đồ

Tất cả sơ đồ là cú pháp Mermaid 10+. Để preview:

- **GitHub**: render native khi xem file này.
- **VS Code**: install extension "Markdown Preview Mermaid Support".
- **Edit live**: paste code-block bất kỳ vào [mermaid.live](https://mermaid.live).
- **Export PNG/SVG**: dùng Mermaid CLI (`@mermaid-js/mermaid-cli`) hoặc button download của `mermaid.live`.

Updates: edit in place. Sơ đồ là phần của cùng git diff với code change — nếu module thêm hoặc flow đổi, update sơ đồ tương ứng trong cùng PR.
