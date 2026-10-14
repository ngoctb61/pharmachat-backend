# PharmaChat Skill Catalog

> 3 skills Phase 1 (prompt/process knowledge)
> Moi skill la mot SKILL.md trong thu muc nay

---

## Phase 1 — Prompt / Process knowledge (3)

| Skill | Agents | File |
|---|---|---|
| Knowledge Base Duoc Pham | Chat Agent | knowledge-base-duoc-pham.md |
| 30 Chat Scenarios Duoc Pham | Chat Agent | chat-scenarios-duoc-pham.md |
| Compliance Rules | Chat Agent | compliance-rules.md |

---

## Agent -> Skill matrix

| Agent | Phase 1 skills |
|---|---|
| Chat Agent | knowledge-base-duoc-pham, chat-scenarios-duoc-pham, compliance-rules |

---

## Skill File Format

Moi skill la mot file Markdown voi frontmatter:

`yaml
---
skill: skill-name
agents: [Chat Agent]
owner: Admin
updated: 2026-09-09
---
`

Noi dung skill mo ta:
1. Purpose — muc dich skill
2. Rules — cac rule phai tuan thu
3. Examples — vi du cu the
4. Edge cases — cac truong hop dac biet
