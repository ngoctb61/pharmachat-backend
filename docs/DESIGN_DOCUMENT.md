# Design Document — PharmaChat

> **Luu y (2026-10-15):** Tai lieu nay viet cho ban .NET. Phan nghiep vu (luong chinh, AI pipeline,
> compliance, self-learning, UI) van dung. Phan kien truc, tech stack va database schema da bi thay the
> boi `docs/MIGRATION_GO_BACKEND.md` va `migrations/`. Khi hai tai lieu mau thuan, theo ke hoach Go.

> Phien ban: 0.1
> Ngay tao: 2026-09-09
> Trang thai: APPROVED (sau brainstorming 25 cau)

---

## 1. Tong quan he thong

PharmaChat la he thong AI Agent Chat tu dong 100% cho cong ty duoc pham. Khach hang gui tin nhan qua Facebook Messenger/Instagram DM, AI tu van san pham 24/7. Sale quan ly tren platform UI.

### Luong trinh chinh

`
Khach hang (Messenger/Instagram)
        ↓ tin nhan
    Meta Business Suite
        ↓ webhook (HMAC-SHA256 verify)
    PharmaChat.Api
        ↓ forward to AgentService
    PharmaChat.AgentService (gRPC)
        ↓
    ┌─────────────────────────────┐
    │ 1. PII Redaction           │
    │ 2. Intent Classification   │
    │ 3. Compliance Check        │
    │ 4. RAG Retrieval (Qdrant)  │
    │ 5. Contact Memory Search   │
    │ 6. LLM Call (Claude)       │
    │ 7. Compliance Filter       │
    │ 8. Self-learning Analysis  │
    └─────────────────────────────┘
        ↓ reply
    Meta Graph API → Khach hang
        ↓ log
    SQL Server (messages) + Qdrant (chat vectors)
`

---

## 2. Kien truc (Architecture)

### 2.1 Clean Architecture 4 lop + gRPC

`
PharmaChat/
├── src/
│   ├── shared/
│   │   ├── PharmaChat.Domain/           # Domain entities, zero deps
│   │   ├── PharmaChat.Application/      # MediatR handlers, validators
│   │   ├── PharmaChat.Infrastructure/   # EF Core, Redis, Qdrant, Meta API
│   │   └── PharmaChat.SharedKernel/     # Abstractions, interfaces
│   ├── api/
│   │   ├── PharmaChat.Api/              # ASP.NET Core minimal APIs
│   │   └── PharmaChat.Api.Contracts/    # Public DTOs
│   ├── agents/
│   │   ├── PharmaChat.Agents.Core/      # IAgent, ChatAgent, RAG, Skills
│   │   └── PharmaChat.AgentService/     # gRPC host (1 agent: Chat)
│   └── frontend/
│       └── pharmachat-web/              # React 19 + Vite + Tailwind
├── proto/                               # gRPC contracts (.proto)
├── deploy/                              # Docker, migrations, seed data
├── tests/                               # xUnit, Testcontainers
└── .sdd/                                # SDD artifacts
`

### 2.2 Docker Containers

| Container | Port | Mo ta |
|---|---|---|
| sqlserver | 1433 | SQL Server 2022 |
| qdrant | 6333 | Vector store |
| redis | 6379 | Cache + SignalR backplane |
| pharmachat-api | 5000 | Backend API |
| pharmachat-agent | 5001 | gRPC Agent Service |
| frontend | 3000 | React UI |

---

## 3. Database Schema (SQL Server 2022)

### 3.1 Tables (12 tables)

`
tenants                    # Multi-tenant config
users                      # Admin, Sale
roles / user_roles         # RBAC
conversations              # Cuoc tro chuyen (from Meta)
messages                   # Tin nhan (immutable)
contacts                   # Khach hang
contact_memories           # AI memory ve KH (vector)
kb_modules                 # Knowledge Base modules
kb_versions                # KB versions (content_md + embedding)
kb_test_cases              # Accuracy test set
compliance_logs            # Audit trail compliance
agent_sessions             # AI session tracking
agent_traces               # Immutable agent logs
`

### 3.2 Entity Relationships

`
tenants 1──N users
tenants 1──N conversations
conversations 1──N messages
conversations N──1 contacts
contacts 1──N contact_memories
tenants 1──N kb_modules
kb_modules 1──N kb_versions
kb_modules 1──N kb_test_cases
conversations 1──N compliance_logs
conversations 1──N agent_sessions
agent_sessions 1──N agent_traces
`

---

## 4. AI Chat Agent Pipeline


### 3.3 LLM & Embedding Config (tu admin UI)

| Bang | Muc dich |
|---|---|
| llm_configs | Cau hinh Chat/LLM — provider, model, API key (AES encrypted), temperature, max_tokens |
| embedding_configs | Cau hinh Embedding — provider, model, API key (AES encrypted), dimension |

**Quy tac:**
- 1 tenant co the co nhieu config, chi 1 config active (is_active = 1) cho moi loai
- Admin cau hinh tu UI — API key duoc AES-256 encrypt truoc khi luu DB
- Co the ca 2 dung cung 1 model (vd: GPT-6 Astra) neu model ho tro ca chat + embedding
- Tach 2 bang de flexibility: ngay mai doi embedding sang Cohere embed-v3 khong anh huong chat

**Flow su dung trong code:**
1. ChatAgent nhan tin nhan → query llm_configs WHERE tenant_id = X AND is_active = 1
2. Giai ma api_key_encrypted → goi LLM API
3. Query embedding_configs WHERE tenant_id = X AND is_active = 1
4. Giai ma api_key_encrypted → goi Embedding API → tao vector
5. Search Qdrant voi vector result


### 3.4 LLM Config Schema

`sql
llm_configs (
    id                  UNIQUEIDENTIFIER PK DEFAULT NEWID(),
    tenant_id           UNIQUEIDENTIFIER NOT NULL FK -> tenants,
    provider            NVARCHAR(32) NOT NULL,       -- anthropic | openai | openrouter
    model_id            NVARCHAR(128) NOT NULL,      -- claude-sonnet-4-6 | gpt-6-astra
    api_key_encrypted   NVARCHAR(MAX) NOT NULL,      -- AES-256 encrypted
    base_url            NVARCHAR(512),                -- custom endpoint
    max_tokens          INT DEFAULT 4096,
    temperature         DECIMAL(3,2) DEFAULT 0.7,
    is_active           BIT DEFAULT 1,
    created_at          DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET(),
    updated_at          DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET()
)
`

### 3.5 Embedding Config Schema

`sql
embedding_configs (
    id                  UNIQUEIDENTIFIER PK DEFAULT NEWID(),
    tenant_id           UNIQUEIDENTIFIER NOT NULL FK -> tenants,
    provider            NVARCHAR(32) NOT NULL,       -- openai | cohere | local
    model_id            NVARCHAR(128) NOT NULL,      -- text-embedding-3-large | gpt-6-astra
    api_key_encrypted   NVARCHAR(MAX) NOT NULL,      -- AES-256 encrypted
    base_url            NVARCHAR(512),                -- custom endpoint
    dimension           INT NOT NULL DEFAULT 3072,    -- so chieu vector
    is_active           BIT DEFAULT 1,
    created_at          DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET(),
    updated_at          DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET()
)
`

### 3.6 AI Config Flow

`
Admin UI: Cau hinh AI
├── Chat/LLM
│   ├── Provider: [Anthropic ▼]
│   ├── Model: [claude-sonnet-4-6]
│   ├── API Key: [sk-ant-...]
│   ├── Base URL: [https://api.anthropic.com]
│   ├── Temperature: [0.7]
│   └── Max Tokens: [4096]
│
├── Embedding
│   ├── Provider: [OpenAI ▼]
│   ├── Model: [text-embedding-3-large]
│   ├── API Key: [sk-...]
│   ├── Base URL: [https://api.openai.com]
│   └── Dimension: [3072]
│
└── [Luu] [Kiem tra ket noi] [Dat lam mac dinh]
`

**Note:** Co the ca 2 dung cung 1 model (vd: GPT-6 Astra) neu model ho tro ca chat + embedding.

### 4.1 Request Flow

`
1. Receive message (from Meta webhook)
2. Resolve tenant (from conversation)
3. Load conversation history (SQL Server)
4. Load contact memory (Qdrant)
5. PII Redaction (Presidio)
6. Intent Classification (keyword + LLM)
7. Compliance Pre-check (block Rx advice)
8. RAG Retrieval:
   a. Embed query (text-embedding-3-large)
   b. Search Qdrant (cosine similarity, top-5)
   c. Load contact memory (top-3 relevant)
   d. Build context = KB chunks + memory + history
9. LLM Call (Claude Sonnet 4.6 via Semantic Kernel)
10. Compliance Post-check (disclaimer injection, severity detection)
11. Self-learning Analysis (pattern detection)
12. Send reply (Meta Graph API)
13. Log: messages, agent_traces, compliance_logs
14. Embed reply + store in Qdrant (chat history vectors)
`

### 4.2 System Prompt

`
Ban la [Ten], Chuyen gia Duoc si, 30-35 tuoi, nu.
Ban tu van ban hang cho cong ty duoc pham Huyen Song Khoe.

QUY TAC:
- Luon tra loi confident, giong nguoi that
- KHONG bao loi "minh khong chac chan"
- Luon them disclaimer "Tham khao y kien bac si truoc khi dung"
  khi goi den san pham
- KHONG tu van thuoc ke don
- KHONG dam bao chua benh
- Neu khach mo ta trieu chung nang -> tu van lien he bac si
- Tra loi ngan gon, than thien, chuyen nghiep
- Dung gioi thieu san pham khi phu hop voi nhu cau khach
`

### 4.3 RAG Strategy

| Phase | Strategy |
|---|---|
| **Embedding** | text-embedding-3-large (3072 dimensions) |
| **Vector Store** | Qdrant Docker |
| **Chunk Size** | ~1000 ky tu/chunk |
| **Retrieval** | Cosine similarity, top-5 |
| **Context** | KB chunks + Contact memory + Conversation history |
| **Fallback** | Keyword search (neu Qdrant loi) |

### 4.4 Conversation Memory

| Type | Luu tru | Su dung |
|---|---|---|
| **Full history** | SQL Server (messages) | AI doc toan bo cuoc tro chuyen |
| **Chat vectors** | Qdrant | Tim kiem lich su theo noi dung |
| **Contact memory** | Qdrant | AI nho thong tin KH (ten, benh, san pham) |
| **Summary** | Qdrant | Tóm tat cuoc tro chuyen dai (> 50 tin) |

---

## 5. Meta Business Suite Integration

### 5.1 OAuth Flow

`
1. Admin click "Connect Meta" → redirect to Facebook OAuth
2. Facebook callback → exchange code for System User Token
3. Store encrypted token in meta_connections
4. Sync pages from Graph API → meta_assets
5. Register webhook for each page
`

### 5.2 Webhook Processing

`
1. Receive webhook POST /api/webhooks/meta
2. Verify HMAC-SHA256 signature (FixedTimeEquals)
3. Parse message/comment payload
4. Resolve conversation (by page_id + thread_id)
5. Create/update message in DB
6. Forward to AI Agent (gRPC)
7. AI generates reply
8. Send reply via Graph API
9. Log in agent_traces
`

### 5.3 Message Send

`
1. AI generates reply text
2. Compliance check (disclaimer, block Rx)
3. Rate limit check (Meta API limits)
4. Send via Graph API POST /me/messages
5. Retry on 429/5xx (exponential backoff)
6. Log sent_at timestamp
`

---

## 6. Self-learning System

### 6.1 Pattern Detection

`
Every 24h (background job):
1. Query chat logs from last 24h
2. Cluster questions by intent
3. Find questions asked > 3 times but not in KB
4. Generate draft FAQ entry
5. Store in kb_suggestions table
6. Admin toggle: auto-approve or manual review
`

### 6.2 Admin Toggle

| Mode | Behavior |
|---|---|
| **Tu dong duyet** | AI tu them vao KB, KHONG can admin approve |
| **Admin duyet** | AI de nghi, hien trong admin UI, admin click "Approve" |

---

## 7. Compliance Gate

### 7.1 Rules

| Rule | Kich hoat | Hanh dong |
|---|---|---|
| DISCLAIMER_REQUIRED | Response goi den san pham | Them "Tham khao y kien bac si..." |
| RX_BLOCKED | Khach hoi ve thuoc ke don | "Chung toi chi tu van san pham chuc nang..." |
| SEVERITY_ESCALATED | Khach mo ta trieu chung nang | "Vui long lien he bac si ngay..." |
| CURE_CLAIM_BLOCKED | AI phan hoi dam bao chua benh | Block + regenerate |

### 7.2 Compliance Log

Moi hanh dong compliance duoc luu:
- conversation_id
- rule_code
- action (inject_disclaimer | block | escalate)
- created_at

---

## 8. UI Design (tu screenshot)

### 8.1 Layout

3-panel layout:
- **Left Sidebar** (200px, dark): Navigation menu
- **Middle** (flex): Conversation list + Chat area
- **Right Sidebar** (300px): Customer Memory Panel

### 8.2 Components

| Component | Mo ta |
|---|---|
| Sidebar | Nav menu: Hoi thoai, Khach hang, San pham, Don hang, Bao cao. AI ON toggle |
| ConversationList | Danh sach KH, ten, tin nhan gan nhat, "AI dang tu van", thoi gian |
| ChatArea | Tin KH (trai), Tin AI (phai), "Dang suy nghi..." trang thai, Input box |
| CustomerMemory | Ho so KH: ten, tuoi, SDT, tong chi tieu. Bo nho AI: benh, san pham, lich su |

### 8.3 Colors

| Element | Mau |
|---|---|
| Primary | Xanh la (#22C55E) |
| Background | Trang nhat (#F8FAFC) |
| Sidebar | Trang hoac xam nhat |
| AI Message | Xanh nhat (#DCFCE7) |
| Customer Message | Trang |
| Badges | Xanh la (AI active), Vang (can xu ly), Do (can nhan vien) |

---

## 9. Tech Stack

| Lop | Cong nghe |
|---|---|
| Backend API | .NET 8, ASP.NET Core minimal APIs, EF Core 8, MediatR, FluentValidation |
| Agent Service | .NET 8 gRPC, Microsoft Semantic Kernel |
| LLM | Anthropic Claude Sonnet 4.6 |
| Embedding | OpenAI text-embedding-3-large (3072 dim) |
| Database | SQL Server 2022 |
| Vector Store | Qdrant Docker |
| Cache | Redis 7 |
| Frontend | React 19, Vite, TypeScript, Tailwind, Zustand, TanStack Query |
| Auth | ASP.NET Core Identity + JWT Bearer |
| Real-time | SignalR |
| Deploy | Docker Compose (1 VPS 8GB RAM) |

---

## 10. NFR

| ID | Requirement | Target |
|---|---|---|
| NFR-01 | Chat reply latency p95 | < 5 seconds |
| NFR-02 | RAG retrieval p95 | < 200ms |
| NFR-03 | Conversation list load p95 | < 500ms |
| NFR-04 | Message send latency p95 | < 3 seconds |
| NFR-05 | KB accuracy | >= 85% on test set |
| NFR-06 | LLM cost cap | /month/tenant |
| NFR-07 | Uptime | 99.5% |
| NFR-08 | Compliance check p95 | < 500ms |




### 5.3 Meta Business Suite Integration

**Architecture:** Webhook (inbound) + Send API (outbound) + SignalR (real-time)

`
INBOUND (Webhook — real-time < 1s):
KH gui tin → Meta webhook → PharmaChat verify HMAC → Parse message → Luu DB → SignalR push

OUTBOUND (Send API — ~1-3s):
AI tra loi → Goi Graph API /me/messages → Meta gui KH (~1-3s)
`

**Toc do tong:** KH gui tin → AI tra loi: ~3-5 giay. Sale thay tin nhan: < 1 giay (SignalR).

**Webhook Events subscribed:**
- messages — Tin nhan moi tu KH
- messaging_postbacks — Quick reply button click
- messaging_optins — KH opt-in nhan tin
- feed — Comment tren fanpage

**Webhook Verification:**
- GET /api/webhooks/meta?hub.mode=subscribe&hub.verify_token={TOKEN}&hub.challenge={CHALLENGE}
- Response: {CHALLENGE} neu verify_token khop

**Webhook Handler:**
- POST /api/webhooks/meta
- Header: X-Hub-Signature-256 = sha256(app_secret + request_body)
- Verify HMAC → Parse JSON → Extract messaging[] → Create messages → Return 200 OK

**Send Message API:**
- POST https://graph.facebook.com/v19.0/me/messages
- Header: Authorization: Bearer {PAGE_ACCESS_TOKEN}
- Body: { "recipient": { "id": "{PSID}" }, "message": { "text": "..." } }
- Response: { "message_id": "..." }

**SignalR Hub (Sale UI):**
- JoinConversation — Sale vao xem conversation
- SendMessage — Sale gui reply
- ReceiveMessage — Nhan tin nhan moi (tu KH hoac AI)
- TypingIndicator — AI dang viet
- NewConversation — Co conversation moi
