# SPEC-06 - Inbox (Sale Platform)

Status: DRAFT
Traces to: FR-06, UC-F01..F08

---

## 1. Business Context
Sale can platform UI de theo doi AI tu van 24/7. Layout 3-panel: Sidebar (nav) + Middle (list + chat) + Right (customer memory). Moi sale thay toan bo inbox cong ty. AI 100% tu dong, sale chi xem va theo doi.

## 2. User Stories
- AS A Sale I WANT xem toan bo inbox cua cong ty SO THAT toi biet AI dang tu van nhung khach nao.
- AS A Sale I WANT xem tin nhan khach gui va AI tra loi SO THAT toi theo doi chat luong.
- AS A Sale I WANT thay AI nho gi ve KH SO THAT toi hieu ngu canh tu van.
- AS A Admin I WANT quan ly KB va cau hinh agent SO THAT AI hoat dong tot.

## 3. Acceptance Criteria (EARS)
- THE SYSTEM SHALL display 3-panel layout: sidebar + conversation list + chat + customer memory.
- THE SYSTEM SHALL display conversations list sorted by last_message_at DESC.
- THE SYSTEM SHALL show "AI dang tu van" badge tren moi conversation.
- WHEN a sale views a conversation THE SYSTEM SHALL show all messages voi timestamps.
- WHEN AI is generating reply THE SYSTEM SHALL display "Dang suy nghi..." trang thai.
- THE SYSTEM SHALL display Customer Memory Panel ben phai: ten, tuoi, SDT, tong chi tieu, benh tien su, san pham da mua.
- THE SYSTEM SHALL support conversation filters: Tat ca / AI dang chat / Toi xu ly.
- THE SYSTEM SHALL support channel filters: Facebook, Messenger, Instagram.
- WHERE user role is sale THE SYSTEM SHALL show all conversations (khong phan quyen).
- WHEN new message arrives THE SYSTEM SHALL update conversation list real-time via WebSocket.

## 4. API Contracts / Data Models
- GET /api/inbox/conversations - list conversations
- GET /api/inbox/conversations/{id} - get conversation detail + messages
- GET /api/inbox/conversations/{id}/memory - get customer memory
- WebSocket /ws/inbox - realtime new message + typing indicator
- Tables: conversations, messages, contacts, contact_memories

## 5. Technical Constraints
- Layout: 3-panel responsive (mobile: 1-panel stack).
- WebSocket hub for realtime message push.
- React 19 + Vite + Tailwind + Zustand + TanStack Query.
- Colors: Xanh la (#22C55E) primary, Trang nhat (#F8FAFC) background.

## 6. Out of Scope
- Multi-conversation tabs (Phase 2)
- Labels and notes (Phase 2)
- Quick reply templates (Phase 2)
- Command palette (Phase 2)

## 7. NFR
- NFR-03: Conversation list load p95 < 500ms.
- NFR-04: Message send latency p95 < 1 second.
- NFR-10: WebSocket connection established p95 < 2 seconds.

## 8. Error Handling Matrix
| Error | Detection | User-visible | Recovery |
|---|---|---|---|
| WebSocket disconnect | Connection lost | Banner "Dang ket noi lai..." | Auto reconnect |
| Conversation load fail | API error | "Loi tai du lieu" | Refresh page |
| Memory load fail | API error | "Khong the tai bo nho" | Retry |

## 9. Open Questions
| Item | Owner | Due | Status |
|---|---|---|---|
| Mobile layout priority? | Frontend | T3 | open |
