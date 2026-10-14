# SPEC-04 - Meta Business Suite Integration

Status: DRAFT
Traces to: FR-04, UC-D01..D08

---

## 1. Business Context
He thong can nhan va gui tin nhan tu Facebook Messenger va Instagram DM thong qua Meta Business Suite. Day la channel duy nhat de khach hang tiep can AI Agent. Tich hop chi cho chat — khong post, khong ads, khong content.

## 2. User Stories
- AS A Admin I WANT ket noi Meta App voi he thong SO THAT tin nhan tu Messenger/Instagram duoc dong bo.
- AS A Khach hang I WANT gui tin nhan qua Messenger SO THAT AI tra loi ngay lap tuc.
- AS A Sale I WANT thay tin nhan khach gui tren platform SO THAT toi theo doi chat luong.

## 3. Acceptance Criteria (EARS)
- WHEN Admin connects Meta App via OAuth THE SYSTEM SHALL exchange code for System User Token and store encrypted.
- WHEN a webhook message arrives from Meta THE SYSTEM SHALL verify HMAC-SHA256 signature before processing.
- WHEN a new message arrives THE SYSTEM SHALL create/update conversation and forward to AI Agent.
- WHEN AI Agent generates reply THE SYSTEM SHALL send via Meta Graph API within 3 seconds.
- THE SYSTEM SHALL sync page list from Meta and store in meta_assets table.
- WHEN token expires or becomes invalid THE SYSTEM SHALL mark status as reconnect_required and alert Admin.
- THE SYSTEM SHALL handle both text messages and quick replies.
- THE SYSTEM SHALL NOT post content, run ads, or publish to Facebook/Instagram.

## 4. API Contracts / Data Models

### 4.1 Admin API
- POST /api/admin/meta/config - { appId, appSecret, configurationId, redirectUri }
- POST /api/admin/meta/connect - initiate OAuth flow
- GET /api/admin/meta/callback - OAuth callback
- POST /api/admin/meta/sync - sync pages from Meta
- POST /api/admin/meta/validate - validate current connection
- DELETE /api/admin/meta - disconnect
- GET /api/admin/meta/status - connection status + assets

### 4.2 Webhook API
- GET /api/webhooks/meta - verify webhook (hub.mode, hub.verify_token, hub.challenge)
- POST /api/webhooks/meta - receive webhook from Meta

### 4.3 Tables
- meta_configs: Cau hinh Meta App (app_id, app_secret_enc, config_id, webhook_token)
- meta_connections: Ket noi (client_business_id, system_user_id, access_token_enc, status)
- meta_assets: Pages (external_id, name, access_token_enc, is_default)
- meta_oauth_states: OAuth flow state

## 5. Webhook + Send API Architecture

### 5.1 Inbound (Webhook — real-time < 1s)

**Flow:**
`
KH gui tin nhan tren Messenger
        ↓ (< 1s)
Meta Graph API → POST /api/webhooks/meta
        ↓
PharmaChat verify HMAC-SHA256 signature
        ↓
Parse message payload (messaging[])
        ↓
Luu vao DB (messages table, direction=in)
        ↓
Broadcast qua WebSocket → Sale thay ngay
        ↓
Goi AI Agent → AI tra loi
`

**Webhook Verification:**
- GET /api/webhooks/meta?hub.mode=subscribe&hub.verify_token={TOKEN}&hub.challenge={CHALLENGE}
- Response: {CHALLENGE} neu verify_token khop

**Webhook Handler:**
- POST /api/webhooks/meta
- Header: X-Hub-Signature-256 = sha256(app_secret + request_body)
- Verify HMAC → Parse JSON → Extract messaging[] → Create messages → Return 200 OK

**Events subscribed:**
- messages — Tin nhan moi tu KH
- messaging_postbacks — Quick reply button click
- messaging_optins — KH opt-in nhan tin
- feed — Comment tren fanpage

### 5.2 Outbound (Send API — ~1-3s)

**Flow:**
`
AI Agent tra loi
        ↓
PharmaChat → POST Graph API /me/messages
        ↓
Meta gui tin nhan den KH tren Messenger
        ↓ (< 1-3s)
KH nhan tin nhan
`

**Send API Request:**
- POST https://graph.facebook.com/v19.0/me/messages?access_token={PAGE_ACCESS_TOKEN}
- Header: Authorization: Bearer {PAGE_ACCESS_TOKEN}
- Body: { "recipient": { "id": "{PSID}" }, "message": { "text": "..." } }
- Response: { "message_id": "..." }

### 5.3 WebSocket Hub (Sale UI)

**Events:**
- JoinConversation — Sale vao xem conversation
- LeaveConversation — Sale roi conversation
- SendMessage — Sale gui reply (human)
- ReceiveMessage — Nhan tin nhan moi (tu KH hoac AI)
- TypingIndicator — AI dang viet
- NewConversation — Co conversation moi

**Flow:**
1. Sale mo conversation → Client emit: JoinConversation(conversationId) → Server join group: conversation:{id}
2. AI tra loi khach hang → Server emit: ReceiveMessage(conversationId, message) → Tat ca clients trong group nhan
3. Sale gui reply → Client emit: SendMessage(conversationId, content) → Server create message + broadcast
4. AI dang viet → Server emit: TypingIndicator(conversationId, "AI") → Clients hien "Dang suy nghi..."
5. Conversation moi → Server emit: NewConversation(conversation) → Tat ca clients trong tenant group nhan

## 6. Technical Constraints
- Meta Graph API v19.0+
- System User Token (not User Token) for long-lived access.
- Webhook must verify HMAC-SHA256 with FixedTimeEquals.
- Access tokens encrypted with AES-256 before storage.
- Webhook endpoint must respond within 20 seconds.
- Chi ho tro chat — khong post, khong ads.
- Send API latency: ~1-3s (acceptable).

## 7. Out of Scope
- TikTok integration (Phase 2)
- Zalo integration (Phase 2)
- WhatsApp integration (Phase 2)
- Content publishing to Facebook/Instagram (Phase 2)
- Ads management (Phase 2)

## 8. NFR
- NFR-04: Webhook processing p95 < 2 seconds.
- NFR-11: Message send latency p95 < 3 seconds.
- NFR-12: Token refresh job runs every 6 hours.
- NFR-17: WebSocket connection established p95 < 2 seconds.

## 9. Error Handling Matrix
| Error | Detection | User-visible | Recovery |
|---|---|---|---|
| OAuth code invalid | Graph API error | Ket noi that bai | Retry OAuth flow |
| Webhook HMAC fail | Signature mismatch | 401 Unauthorized | Check webhook secret |
| Token expired | Graph API 401 | Can ket noi lai | Auto refresh or manual reconnect |
| Graph API rate limit | 429 response | Retry sau 30s | Exponential backoff |
| Page not found | Graph API 404 | Page khong ton tai | Re-sync pages |
| Send API fail | Graph API error | AI reply khong gui duoc | Retry x3, log error |

## 10. Open Questions
| Item | Owner | Due | Status |
|---|---|---|---|
| Use Basic Display or System User token? | Backend | T1 | resolved — System User Token |
