# SPEC-05 - Compliance Gate

Status: DRAFT
Traces to: FR-05, UC-E01..E05

---

## 1. Business Context
Cong ty duoc pham ban san pham chuc nang/thuc pham bo sung, KHONG ban thuoc ke don. AI Agent phai tu dong: (1) them disclaimer y te, (2) block tu van thuoc ke don, (3) escalate khi khach mo ta trieu chung nang. AI KHONG duoc bao loi "minh khong chac chan" — luon tra loi confident.

## 2. User Stories
- AS A Admin I WANT AI tu dong them disclaimer "Tham khao y kien bac si" SO THAT phap luat duoc dam bao.
- AS A Admin I WANT AI block tu van ve thuoc ke don SO THAT khong vi pham quy dinh.
- AS A Admin I WANT AI phan loai san pham (OTC/TPCN) SO THAT compliance check chinh xac.

## 3. Acceptance Criteria (EARS)
- WHEN AI response mentions a product THE SYSTEM SHALL append disclaimer "Tham khao y kien bac si truoc khi dung" if not already present.
- WHEN user asks about prescription drugs (thuoc ke don, co don thuoc...) THE SYSTEM SHALL respond: "Chung toi chi tu van san pham chuc nang. Vui long lien he bac si de duoc tu van ve thuoc ke don."
- WHEN user describes severe symptoms (dau nguc, kho tho, mat tinh than...) THE SYSTEM SHALL respond: "Vui long lien he bac si ngay de duoc kham va tu van", chuyen hoi thoai sang `human` (handoff_reason = severe_symptom) va bao sale (B-1).
- THE SYSTEM SHALL NOT guarantee cure or treatment effectiveness in any response.
- THE SYSTEM SHALL tag all conversations with compliance_level: normal | disclaimer_required | escalated.
- THE SYSTEM SHALL NOT respond "minh khong chac chan" — luon tra loi confident.

## 4. API Contracts / Data Models
- Compliance middleware runs after AI generates reply, before sending to user.
- Tables: compliance_logs (id, tenant_id, conversation_id, rule_code, action, created_at)
- Rule codes: DISCLAIMER_REQUIRED, RX_BLOCKED, SEVERITY_ESCALATED, CURE_CLAIM_BLOCKED

## 5. Technical Constraints
- Disclaimer injection must not exceed 100 tokens.
- Compliance check must complete within 500ms.
- Severity detection uses keyword matching + LLM classification fallback.
- All compliance actions logged for audit trail.
- Phan loai san pham: OTC (khong ke don) / TPCN (thuc pham bo sung).

## 6. Out of Scope
- Advanced drug interaction checking (Phase 2)
- Regulatory reporting (Phase 2)
- Multi-language compliance (Phase 2)

## 7. NFR
- NFR-08: Compliance check p95 < 500ms.
- NFR-13: 100% of product-mentioning responses include disclaimer.
- NFR-14: Compliance logs retained for 90 days.

## 8. Error Handling Matrix
| Error | Detection | User-visible | Recovery |
|---|---|---|---|
| Disclaimer injection fail | Exception | Reply without disclaimer + alert | Manual review |
| Severity detection uncertain | Low confidence | Escalate (fail-open) | Human review |
| Compliance log write fail | DB exception | Reply still sent | Retry async |

## 9. Open Questions
| Item | Owner | Due | Status |
|---|---|---|---|
| How to handle customer sending prescription photos? | Admin | T2 | resolved 2026-10-15 (B-5): AI khong doc anh, chuyen hoi thoai cho sale |
