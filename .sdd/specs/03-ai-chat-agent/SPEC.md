# SPEC-03 - AI Chat Agent (RAG)

Status: DRAFT
Traces to: FR-03, UC-C01..C10

---

## 1. Business Context
AI Chat Agent la trung tam cua he thong. No su dung RAG de tra loi khach hang dua tren Knowledge Base. Agent tu dong tra loi; khi hoi thoai o che do `human` (sale tiep quan, quyet dinh B-1) thi agent KHONG tra loi. AI phai tra loi confident, giong nguoi that, voi persona "Chuyen gia Duoc si, 30-35 tuoi, nu, than thien + chuyen nghiep".

## 2. User Stories
- AS A Khach hang I WANT nhan tu van san pham ngay lap tuc SO THAT toi khong can cho sale.
- AS A Sale I WANT xem AI tu van chinh xac SO THAT toi theo doi chat luong.
- AS A Admin I WANT AI hoc tu chat logs SO THAT AI tro nen khon hon theo thoi gian.

## 3. Acceptance Criteria (EARS)
- WHEN a user sends a message THE SYSTEM SHALL classify intent within 1 second.
- WHEN intent is product_inquiry THE SYSTEM SHALL retrieve top-5 relevant KB chunks via RAG.
- THE SYSTEM SHALL load toan bo cuoc tro chuyen history lam context.
- THE SYSTEM SHALL load contact memory (ten, SDT, benh, san pham da mua) tu pgvector.
- WHEN response contains medical disclaimer requirement THE SYSTEM SHALL append disclaimer automatically.
- WHEN user describes severe symptoms (dau nguc, kho tho nghiem trong...) THE SYSTEM SHALL respond: "Vui long lien he bac si ngay".
- THE SYSTEM SHALL NOT respond "minh khong chac chan" — luon tra loi confident.
- THE SYSTEM SHALL log all agent traces to agent_traces table (immutable).
- WHEN monthly LLM cost reaches 80% of cap THE SYSTEM SHALL emit warning alert.
- WHEN monthly LLM cost reaches 100% of cap THE SYSTEM SHALL stop AI replies.

## 4. API Contracts / Data Models
- POST /api/chat/reply - { conversationId, message } -> { reply, intent, citations }
- GET /api/chat/history/{conversationId} - get chat history
- gRPC: ChatAgent.Reply (unary) + ChatAgent.StreamReply (server streaming)
- Tables: conversations, messages, agent_sessions, agent_traces, contact_memories

## 5. Technical Constraints
- LLM: Claude Sonnet 4.6 via anthropic-sdk-go; model va gia lay tu llm_configs.
- Embedding: text-embedding-3-large (3072 dim) via OpenAI.
- RAG: Retrieve from pgvector using cosine similarity.
- Context window: toan bo cuoc tro chuyen + contact memory + KB chunks.
- Summary: tóm tat cuoc tro chuyen dai (> 50 tin) de giam token cost.
- PII redaction before LLM call.
- AI Persona: "Chuyen gia Duoc si", nu 30-35, than thien + chuyen nghiep.

## 6. Out of Scope
- Voice-to-text (Phase 2)
- Multi-language detection (Phase 2)
- Streaming response (chi "Dang suy nghi..." -> ket qua)
- Confidence score display (khong hien "96% tin cay")

## 7. NFR
- NFR-01: Reply latency p95 < 5 seconds.
- NFR-05: Accuracy >= 85% on test set.
- NFR-06: Cost cap /month/tenant.
- NFR-08: Compliance check p95 < 500ms.

## 8. Error Handling Matrix
| Error | Detection | User-visible | Recovery |
|---|---|---|---|
| LLM timeout | SK exception | "Dang xu ly..." -> "Vui long thu lai" | Retry x2, then log |
| LLM rate limit | 429 response | "Vui long thu lai" | Retry with backoff |
| pgvector down | Connection fail | AI tra loi khong co RAG context | Fallback keyword search |
| Cost cap reached | Tracker check | AI tam dung | Wait next month |
| Toxic content | Filter block | "Tin nhan khong phu hop" | Block + log |

## 9. Open Questions
| Item | Owner | Due | Status |
|---|---|---|---|
| Summary strategy cho cuoc tro chuyen > 100 tin? | Backend | T2 | resolved 2026-10-15 (B-6): tom tat cuon chieu, giu 50 tin gan nhat |
