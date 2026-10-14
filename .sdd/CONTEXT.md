# CONTEXT.md — PharmaChat: AI Agent Chat cho cong ty duoc pham

> Phien ban: 0.3 (cap nhat quyet dinh B-1..B-6 khi chuyen sang Go, xem docs/MIGRATION_GO_BACKEND.md)
> Ngay tao: 2026-09-09
> Cap nhat: 2026-10-15
> Trang thai: APPROVED

---

## 1. San pham la gi?

**PharmaChat** la he thong AI Agent Chat su dung RAG (Retrieval-Augmented Generation) de tu van ban hang va cham soc khach hang 24/7 cho cong ty duoc pham. AI tu dong tra loi; sale co the tiep quan hoi thoai khi can (quyet dinh B-1).

### Doanh nghiep
- **Ten**: Huyen Song Khoe (https://www.facebook.com/huyensongkhoe)
- **Nganh**: Duoc pham — san pham chuc nang, thuc pham bo sung (OTC/TPCN)
- **Quy mo**: Nho-trung binh, 5-10 nhan vien sale
- **San pham chinh**:
  1. Boganic Premium (Traphaco): https://s.shopee.vn/5LBjm4e8PU
  2. San pham 2: https://s.shopee.vn/Ln3oR7rRj
- **So luong san pham**: Khong gio han — admin them bat cu luc nao qua KB

### Giai phap
- AI Agent Chat hoat dong 24/7, tu dong tu van; sale tiep quan khi can, va hoi thoai tu chuyen sang sale khi co trieu chung nang, khi khach gui anh hoac khi cham han muc chi phi (B-1, B-5)
- Tich hop Meta Business Suite (Facebook Messenger, Instagram DM) chi cho chat
- RAG dua tren Knowledge Base + pgvector vector store
- Self-learning: AI hoc tu chat logs, tu dong cap nhat KB
- Platform UI cho sale quan ly cuoc tro chuyen

---

## 2. Ai su dung?

| Vai tro | Mo ta | Pham vi |
|---|---|---|
| **Khach hang** | Nguoi dung cuoi, truy cap qua Facebook Messenger/Instagram DM | Gui tin nhan, nhan tu van tu AI. KHONG truy cap platform |
| **Sale** | Nhan vien ban hang | Dang nhap platform, xem toan bo inbox, theo doi AI tu van |
| **Admin** | Quan ly he thong | Quan ly KB, cau hinh agent, approve/tu dong self-learning |

---

## 3. Van de can giai quyet?

1. **Sale qua tai**: 30-50 khach/ngay, phan hoi cham, miss tin nhan
2. **Khong 24/7**: Khach hang nhan tin ban dem/ngay le, khong co ai tra loi
3. **Sai thong tin**: Sale moi thuong khong biet het san pham, tu van sai
4. **Compliance**: Khong co co che tu dong canh bao y te
5. **Khong track**: Khong biet khach hang nao dang nong, nao da tu van

---

## 4. Channel nao?

**Meta Business Suite** — chi 1 channel nhung 2 nen tang:
- Facebook Messenger (chinh)
- Instagram DM (phu)

Tich hop qua Meta Graph API, webhook, OAuth System User Token. **Chi dung cho chat** — khong post, khong ads, khong content.

---

## 5. AI Agent lam gi?

| Chuc nang | Mo ta |
|---|---|
| **Tu van san pham** | Gioi thieu san pham phu hop dua tren trieu chung nhu cau khach |
| **Tra loi FAQ** | Tra loi cac cau hoi pho bien ve san pham, gia ca, chinh sach |
| **Compliance** | Tu dong canh bao "tham khao y kien bac si" khi can thiet |
| **Self-learning** | Phan tich chat logs, thay pattern moi, cap nhat KB tu dong |
| **Nho KH** | Contact memory: ten, SDT, can benh, san pham da mua, lich su chat |

### AI Persona
- **Gioi tinh**: Nu, 30-35 tuoi
- **Vai tro**: Chuyen gia Duoc si
- **Gioi thieu**: Than thien va chuyen nghiep
- **Quan trong**: KHONG bao loi "minh khong chac chan" — luon tra loi confident, giong nguoi that
- **Tone**: Nhu nhan vien ban hang that, khong phat hien la AI

---

## 6. Knowledge Base gom gi?

| Module | Noi dung |
|---|---|
| **KB-01: San pham** | Ten, thanh phan, cong dung, lieu dung, chi dinh, chong chi dinh, tac dung phu, gia |
| **KB-02: FAQ** | Cau hoi pho bien: cach dung, bao quan, han su dung, van chuyen |
| **KB-03: Chinh sach** | Doi tra, van chuyen, thanh toan, bao mat |
| **KB-04: Compliance** | Canh bao y te, quy dinh quang cao thuoc, phan loai san pham (OTC/TPCN) |
| **KB-05: Chat History** | Cac mau chat thanh cong tu sale (grounding cho RAG) |

### RAG Pipeline
`
Upload .txt/.md → Chunk (~1000 ky tu) → Embed (text-embedding-3-large, 3072 dim)
→ Upsert pgvector (metadata: tenant_id, module_code, version_id, snippet)
→ Query: Embed search → Top-K cosine similarity → LLM tra loi voi context
`

---

## 7. Compliance can gi?

| Rules | Chi tiet |
|---|---|
| **Bat buoc disclaimer** | Moi phan hoi goi den san pham phai co "Tham khao y kien bac si truoc khi dung" |
| **Block thuoc ke don** | AI khong duoc tu van ve thuoc ke don (can don thuoc) |
| **Khong dam bao chua benh** | AI khong duoc hua hanh "chua benh", "dam bao co hieu qua" |
| **Trieu chung nang** | Neu khach mo ta trieu chung nang (dau nguc, kho tho nghiem trong...) -> escalate ngay |
| **Muc do san pham** | Chi tu van san pham chuc nang/thuc pham bo sung, KHONG tu van thuoc |
| **Phan loai** | San pham duoc phan loai: OTC (khong ke don) / TPCN (thuc pham bo sung) |

---

## 8. Muc tieu kinh doanh?

| Muc tieu | Chi so | Gia tri hien tai | Muc tieu |
|---|---|---|---|
| **Phan hoi nhanh** | Thoi gian phan hoi binh quan | 15-30 phut | < 5 giay (AI) |
| **Ty le tra loi** | % tin nhan duoc tra loi | 60-70% | 100% (AI 24/7) |
| **Conversion** | % khach hang tu van thanh dat hang | Khong do duoc | Tang 20% |
| **24/7** | Gio phuc vu | 8h/ngay | 24/7 |
| **Sale productivity** | So khach/sale/ngay | 30-50 | 50-80 voi AI ho tro |

---

## 9. Budget/Limit?

| Hang muc | Gia tri |
|---|---|
| **LLM API** | Claude Sonnet 4.6, hard cap $200/thang/tenant (B-4) |
| **Embedding** | text-embedding-3-large (OpenAI), ~.01-0.05/thang (dataset nhe) |
| **Hau tai** | 1 VPS 8GB RAM |
| **Database** | PostgreSQL 17 + pgvector |
| **Vector store** | pgvector (Postgres) |
| **Frontend** | React 19 + Vite + Tailwind |
| **Auth** | JWT + argon2id |

---

## 10. Conversation Memory Strategy

### Context Window
- **Toan bo cuoc tro chuyen** — AI doc het lich su tin nhan
- **Summary + Full history**: AI tóm tat cuoc tro chuyen cu (> 50 tin), giu 50 tin gan nhat. Co the tim kiem toan bo qua vector search

### Contact Memory
- AI nho thong tin ve KH: ten, SDT, can benh, san pham da mua, ngay sinh
- Luu trong pgvector voi metadata: tenant_id, contact_id, type (symptom|product|preference)
- Tim kiem qua vector search khi can thiet

### Self-learning Loop
`
Chat logs → AI phan tich → Pattern detection (NHIEU KH hoi "ngua", "dau bung")
    ↓
Admin toggle: Tu dong duyet hoac Admin approve
    ↓
Cap nhat KB → Embed → Upsert pgvector → AI hoc moi
`

---

## 11. UI Requirements (tu screenshot)

### Layout: 3-panel
`
+------------------+---------------------------+-------------------+
| LEFT SIDEBAR     | MIDDLE: LIST + CHAT       | RIGHT: PROFILE    |
| (nav, dark)      | (flex)                    | (sidebar)         |
|                  |                           |                   |
| AI ON toggle     | Conversation List         | Ho so KH          |
| Khach hang       | - Ten, tin nhan gan nhat  | - Ten, tuoi, SDT  |
| San pham         | - "AI dang tu van"        | - Tong chi tieu   |
| Don hang         | - Thoi gian               | - Don gan nhat    |
| Bao cao          |                           |                   |
|                  | Chat Area                 | BO NHO AI         |
|                  | - Tin KH (trai)           | - Thuoc da mua    |
|                  | - Tin AI (phai)           | - Benh tien su    |
|                  | - "Dang suy nghi..."      | - Hien tai        |
|                  |                           |                   |
|                  | Input box                 |                   |
+------------------+---------------------------+-------------------+
`

### Features
- **3-panel layout**: Sidebar (nav) + Middle (list + chat) + Right (customer memory)
- **AI persona**: Hien "AI dang tu van" tren moi cuoc tro chuyen
- **Customer Memory Panel**: Hien thi AI nho gi ve KH ben phai
- **Chat input**: Text input + "Dang suy nghi..." trang thai
- **Conversation filters**: Tat ca / AI dang chat / Toi xu ly
- **Channel filters**: FB, Messenger, Instagram icons
- **Mau sac**: Xanh la (#22C55E) primary, Trang nhat background

---

## 12. Pham vi & Ranh gioi

### Co trong pham vi (In Scope)
- AI Agent Chat voi RAG (100% tu dong)
- Meta Business Suite integration (Messenger + Instagram) — chi chat
- Knowledge Base CRUD + versioning (Markdown editor + upload .txt/.md)
- Compliance gate (disclaimer, block Rx)
- Multi-tenant (1 tenant duoc pham)
- Self-learning (FAQ + Pattern detection + Admin toggle)
- Contact memory + chat history vector search
- Platform UI (3-panel: inbox + chat + customer memory)
- Admin dashboard don gian

### Khong trong pham vi (Out of Scope — Phase 2)
- Content marketing automation
- Ads automation
- Lead CRM day du
- Doc generation (don thuoc, bao gia)
- Analytics/Bao cao phuc tap
- Zalo/TikTok integration
- Voice-to-text
- Streaming response (chi "Dang suy nghi..." -> ket qua)

---

## 13. Nhom thuc hien

| Vai tro | So luong |
|---|---|
| Backend developer | 1 |
| Frontend developer | 1 |
| AI/ML | 1 (co the la backend developer) |
| QA | 1 (ban than) |

Tong: 2-3 nguoi, 40-60h cho MVP.


## 26. AI Config (tu admin UI)

Admin cau hinh LLM va Embedding tu UI, khong hardcode trong code:

| Bang | Muc dich |
|---|---|
| llm_configs | Chat/LLM: provider, model, API key (AES encrypted), temperature, max_tokens |
| embedding_configs | Embedding: provider, model, API key (AES encrypted), dimension |

- **Tach 2 bang**: de flexibility, debug, cost tracking
- **Co the dung cung 1 model**: neu GPT-6 Astra ho tro ca chat + embedding
- **API key AES-256**: encrypt truoc khi luu DB
- **Admin UI**: 1 form cau hinh ca LLM va Embedding
- **Active config**: chi 1 config active (is_active = 1) cho moi loai
- **Real-time**: khi admin save, config duoc su dung ngay cho request tiep theo


## 27. Meta Integration — Webhook + Send API

**Inbound:** Webhook (real-time < 1s) — Meta push tin nhan den PharmaChat khi KH gui tin.
**Outbound:** Send API (~1-3s) — PharmaChat goi Graph API de gui tin nhan den KH.
**Real-time:** WebSocket push tin nhan den Sale UI ngay lap tuc.

- Webhook verify: HMAC-SHA256 voi FixedTimeEquals
- Send API: POST /me/messages voi PAGE_ACCESS_TOKEN
- Khong the dung Webhook cho outbound — Webhook chi cho inbound
- Toc do tong: KH gui tin → AI tra loi: ~3-5 giay
- Sale thay tin nhan: < 1 giay (WebSocket)
