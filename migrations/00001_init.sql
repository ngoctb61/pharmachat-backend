-- PharmaChat initial schema (PostgreSQL 17 + pgvector >= 0.7).
-- Conventions:
--   snake_case identifiers, uuid PKs via gen_random_uuid(), timestamptz everywhere.
--   tenant_id NOT NULL on every tenant-scoped table, isolated by row-level security.
--   Soft delete via deleted_at on aggregate roots.
--   Embeddings are halfvec(3072): the vector type cannot be HNSW-indexed above 2000 dims.
--   The schema owner runs migrations and bypasses RLS; services connect as members of
--   pharmachat_app, which is subject to RLS and has no UPDATE/DELETE on append-only tables.

-- +goose Up

CREATE EXTENSION IF NOT EXISTS vector;

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pharmachat_app') THEN
        CREATE ROLE pharmachat_app NOLOGIN;
    END IF;
END
$$;
-- +goose StatementEnd

-- Tenant of the current transaction, set with set_config('app.tenant_id', ..., true).
-- Unset or empty yields NULL, which matches no rows.
-- +goose StatementBegin
CREATE FUNCTION app_current_tenant() RETURNS uuid
LANGUAGE sql STABLE AS $$
    SELECT NULLIF(current_setting('app.tenant_id', true), '')::uuid
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END
$$;
-- +goose StatementEnd

-- ------------- Tenants & RBAC -------------

CREATE TABLE tenants (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug          text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9-]{2,64}$'),
    display_name  text NOT NULL,
    plan_name     text NOT NULL DEFAULT 'free',
    is_active     boolean NOT NULL DEFAULT true,
    settings_json jsonb NOT NULL DEFAULT '{"monthly_llm_cap_usd": 200, "self_learning_mode": "manual"}',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz
);

CREATE TABLE users (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    display_name        text NOT NULL,
    email               text NOT NULL,
    phone_number        text,
    password_hash       text NOT NULL,
    access_failed_count integer NOT NULL DEFAULT 0,
    lockout_end         timestamptz,
    is_active           boolean NOT NULL DEFAULT true,
    last_login_at       timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz,
    UNIQUE (tenant_id, email)
);
CREATE INDEX ix_users_tenant_created ON users (tenant_id, created_at DESC);

CREATE TABLE roles (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        text NOT NULL,
    description text,
    is_system   boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

-- Global catalogue, not tenant-scoped.
CREATE TABLE permissions (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code        text NOT NULL UNIQUE,
    description text
);

CREATE TABLE role_permissions (
    tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role_id       uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id uuid NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_roles (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id   uuid NOT NULL REFERENCES roles(id),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE refresh_tokens (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  text NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    revoked_at  timestamptz,
    replaced_by uuid REFERENCES refresh_tokens(id),
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_refresh_tokens_user ON refresh_tokens (user_id);

-- ------------- Contacts -------------

CREATE TABLE contacts (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    display_name   text NOT NULL,
    phone          text,
    email          text,
    locale         text NOT NULL DEFAULT 'vi-VN',
    lifetime_score integer NOT NULL DEFAULT 0,
    meta_json      jsonb NOT NULL DEFAULT '{}',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz
);
CREATE INDEX ix_contacts_tenant_created ON contacts (tenant_id, created_at DESC);
CREATE INDEX ix_contacts_tenant_phone ON contacts (tenant_id, phone) WHERE phone IS NOT NULL;
CREATE INDEX ix_contacts_tenant_email ON contacts (tenant_id, email) WHERE email IS NOT NULL;

-- ------------- Conversations & Messages -------------

CREATE TABLE conversations (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id          uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    contact_id         uuid REFERENCES contacts(id),
    platform           text NOT NULL CHECK (platform IN ('messenger', 'instagram')),
    external_thread_id text NOT NULL,
    status             text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
    compliance_level   text NOT NULL DEFAULT 'normal'
        CHECK (compliance_level IN ('normal', 'disclaimer_required', 'escalated')),
    handling_mode      text NOT NULL DEFAULT 'ai' CHECK (handling_mode IN ('ai', 'human')),
    assigned_to        uuid REFERENCES users(id),
    handoff_reason     text CHECK (handoff_reason IN ('manual', 'severe_symptom', 'image', 'cost_cap')),
    handed_off_at      timestamptz,
    last_message_at    timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz,
    UNIQUE (tenant_id, platform, external_thread_id)
);
CREATE INDEX ix_conv_tenant_status_last ON conversations (tenant_id, status, last_message_at DESC);
CREATE INDEX ix_conv_tenant_handling ON conversations (tenant_id, handling_mode, assigned_to);
CREATE INDEX ix_conv_contact ON conversations (contact_id) WHERE contact_id IS NOT NULL;

-- Append-only. Raw content is purged after 30 days by the retention job.
CREATE TABLE messages (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    conversation_id     uuid NOT NULL REFERENCES conversations(id),
    direction           text NOT NULL CHECK (direction IN ('in', 'out')),
    sender_type         text NOT NULL CHECK (sender_type IN ('contact', 'agent', 'user', 'system')),
    sender_user_id      uuid REFERENCES users(id),
    content             text NOT NULL,
    content_type        text NOT NULL DEFAULT 'text' CHECK (content_type IN ('text', 'image', 'quick_reply', 'other')),
    external_message_id text,
    metadata_json       jsonb NOT NULL DEFAULT '{}',
    sent_at             timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_msg_conv_sent ON messages (conversation_id, sent_at DESC);
CREATE INDEX ix_msg_tenant_sent ON messages (tenant_id, sent_at DESC);
CREATE UNIQUE INDEX ux_msg_tenant_external ON messages (tenant_id, external_message_id)
    WHERE external_message_id IS NOT NULL;

-- Kept apart from messages so the retention job can drop them with the raw content.
CREATE TABLE message_embeddings (
    message_id uuid PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    tenant_id  uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    embedding  halfvec(3072) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_msg_emb_hnsw ON message_embeddings USING hnsw (embedding halfvec_cosine_ops);

CREATE TABLE conversation_summaries (
    conversation_id          uuid PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
    tenant_id                uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    summary                  text NOT NULL,
    covered_until_message_id uuid NOT NULL REFERENCES messages(id),
    covered_message_count    integer NOT NULL,
    updated_at               timestamptz NOT NULL DEFAULT now()
);

-- ------------- Contact Memory (AI memory) -------------

CREATE TABLE contact_memories (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    contact_id    uuid NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    memory_type   text NOT NULL CHECK (memory_type IN ('profile', 'symptom', 'product', 'preference')),
    content       text NOT NULL,
    embedding     halfvec(3072),
    confidence    numeric(5,4) NOT NULL DEFAULT 1.0,
    source_msg_id uuid REFERENCES messages(id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_memories_tenant_contact ON contact_memories (tenant_id, contact_id, created_at DESC);
CREATE INDEX ix_memories_tenant_type ON contact_memories (tenant_id, memory_type);
CREATE INDEX ix_memories_hnsw ON contact_memories USING hnsw (embedding halfvec_cosine_ops);

-- ------------- Knowledge Base -------------

CREATE TABLE kb_modules (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    code        text NOT NULL,
    name        text NOT NULL,
    description text,
    owner_role  text,
    status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz,
    UNIQUE (tenant_id, code)
);

CREATE TABLE kb_versions (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    kb_module_id   uuid NOT NULL REFERENCES kb_modules(id) ON DELETE CASCADE,
    version        integer NOT NULL,
    content_md     text NOT NULL,
    accuracy_score numeric(5,2),
    status         text NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'deploying', 'deployed', 'failed', 'archived')),
    last_error     text,
    deployed_at    timestamptz,
    created_by     uuid REFERENCES users(id),
    created_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (kb_module_id, version)
);
CREATE INDEX ix_kb_versions_module_ver ON kb_versions (kb_module_id, version DESC);
CREATE UNIQUE INDEX ux_kb_versions_one_deployed ON kb_versions (kb_module_id) WHERE status = 'deployed';

CREATE TABLE kb_chunks (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    kb_version_id uuid NOT NULL REFERENCES kb_versions(id) ON DELETE CASCADE,
    module_code   text NOT NULL,
    chunk_index   integer NOT NULL,
    content       text NOT NULL,
    embedding     halfvec(3072) NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (kb_version_id, chunk_index)
);
CREATE INDEX ix_kb_chunks_tenant_module ON kb_chunks (tenant_id, module_code);
CREATE INDEX ix_kb_chunks_hnsw ON kb_chunks USING hnsw (embedding halfvec_cosine_ops);
-- Keyword fallback when vector search is unavailable (SPEC-03 error matrix).
CREATE INDEX ix_kb_chunks_fts ON kb_chunks USING gin (to_tsvector('simple', content));

CREATE TABLE kb_test_cases (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    kb_module_id    uuid NOT NULL REFERENCES kb_modules(id) ON DELETE CASCADE,
    question        text NOT NULL,
    expected_answer text NOT NULL,
    is_active       boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_kb_test_module ON kb_test_cases (kb_module_id);

CREATE TABLE kb_suggestions (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    kb_module_id      uuid NOT NULL REFERENCES kb_modules(id) ON DELETE CASCADE,
    intent            text NOT NULL,
    question_sample   text NOT NULL,
    occurrences       integer NOT NULL,
    draft_md          text NOT NULL,
    status            text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'applied')),
    reviewed_by       uuid REFERENCES users(id),
    reviewed_at       timestamptz,
    applied_version_id uuid REFERENCES kb_versions(id),
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_kb_suggestions_tenant_status ON kb_suggestions (tenant_id, status, created_at DESC);

-- ------------- Compliance Logs (append-only) -------------

CREATE TABLE compliance_logs (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    conversation_id uuid NOT NULL REFERENCES conversations(id),
    message_id      uuid REFERENCES messages(id),
    rule_code       text NOT NULL
        CHECK (rule_code IN ('DISCLAIMER_REQUIRED', 'RX_BLOCKED', 'SEVERITY_ESCALATED', 'CURE_CLAIM_BLOCKED')),
    action          text NOT NULL CHECK (action IN ('inject_disclaimer', 'block', 'escalate', 'regenerate')),
    detail_json     jsonb NOT NULL DEFAULT '{}',
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_compliance_tenant_created ON compliance_logs (tenant_id, created_at DESC);
CREATE INDEX ix_compliance_conversation ON compliance_logs (conversation_id);

-- ------------- Agent Sessions & Traces -------------

CREATE TABLE agent_sessions (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    conversation_id uuid REFERENCES conversations(id),
    agent_code      text NOT NULL,
    status          text NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'completed', 'failed', 'blocked')),
    started_at      timestamptz NOT NULL DEFAULT now(),
    finished_at     timestamptz
);
CREATE INDEX ix_agent_sessions_tenant ON agent_sessions (tenant_id, started_at DESC);
CREATE INDEX ix_agent_sessions_conv ON agent_sessions (conversation_id) WHERE conversation_id IS NOT NULL;

-- Append-only.
CREATE TABLE agent_traces (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    session_id  uuid NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    phase       text NOT NULL,
    input_text  text,
    output_text text,
    model_used  text,
    tokens_in   integer,
    tokens_out  integer,
    cost_usd    numeric(12,6),
    latency_ms  integer,
    occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_agent_traces_session ON agent_traces (session_id, occurred_at);

CREATE TABLE llm_usage (
    tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    month         date NOT NULL CHECK (date_trunc('month', month) = month),
    input_tokens  bigint NOT NULL DEFAULT 0,
    output_tokens bigint NOT NULL DEFAULT 0,
    usd_cost      numeric(14,6) NOT NULL DEFAULT 0,
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, month)
);

-- ------------- Meta Integration -------------

CREATE TABLE meta_configs (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      uuid NOT NULL UNIQUE REFERENCES tenants(id) ON DELETE CASCADE,
    app_id         text NOT NULL,
    app_secret_enc text,
    config_id      text,
    webhook_token  text,
    redirect_uri   text,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE meta_connections (
    id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id              uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    client_business_id     text NOT NULL,
    system_user_id         text NOT NULL,
    token_type             text NOT NULL DEFAULT 'business_integration_system_user',
    access_token_enc       text NOT NULL,
    granted_scopes_json    jsonb NOT NULL DEFAULT '[]',
    expires_at             timestamptz,
    data_access_expires_at timestamptz,
    status                 text NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'reconnect_required', 'disconnected')),
    last_error             text,
    last_validated_at      timestamptz,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_meta_conn_tenant ON meta_connections (tenant_id);

CREATE TABLE meta_assets (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    connection_id    uuid NOT NULL REFERENCES meta_connections(id) ON DELETE CASCADE,
    asset_type       text NOT NULL CHECK (asset_type IN ('page', 'instagram_account')),
    external_id      text NOT NULL,
    name             text NOT NULL,
    access_token_enc text NOT NULL,
    is_default       boolean NOT NULL DEFAULT false,
    is_active        boolean NOT NULL DEFAULT true,
    last_synced_at   timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    -- Global: an inbound webhook is routed to its tenant by page / account id.
    UNIQUE (asset_type, external_id)
);
CREATE INDEX ix_meta_assets_tenant ON meta_assets (tenant_id, asset_type);
CREATE INDEX ix_meta_assets_connection ON meta_assets (connection_id);

CREATE TABLE meta_oauth_states (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    state_hash  text NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- ------------- LLM & Embedding Config -------------

CREATE TABLE llm_configs (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    provider              text NOT NULL CHECK (provider IN ('anthropic')),
    model_id              text NOT NULL,
    api_key_encrypted     text NOT NULL,
    base_url              text,
    max_tokens            integer NOT NULL DEFAULT 4096,
    input_usd_per_mtok    numeric(10,4) NOT NULL,
    output_usd_per_mtok   numeric(10,4) NOT NULL,
    is_active             boolean NOT NULL DEFAULT true,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX ux_llm_configs_one_active ON llm_configs (tenant_id) WHERE is_active;

CREATE TABLE embedding_configs (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    provider          text NOT NULL CHECK (provider IN ('openai')),
    model_id          text NOT NULL,
    api_key_encrypted text NOT NULL,
    base_url          text,
    -- Must match the halfvec(3072) columns; changing it requires a re-embed migration.
    dimension         integer NOT NULL DEFAULT 3072 CHECK (dimension = 3072),
    is_active         boolean NOT NULL DEFAULT true,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX ux_embedding_configs_one_active ON embedding_configs (tenant_id) WHERE is_active;

-- ------------- updated_at triggers -------------

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'tenants', 'users', 'contacts', 'conversations', 'contact_memories', 'kb_modules',
        'conversation_summaries', 'llm_usage', 'meta_configs', 'meta_connections', 'meta_assets',
        'llm_configs', 'embedding_configs'
    ] LOOP
        EXECUTE format('CREATE TRIGGER trg_%s_updated_at BEFORE UPDATE ON %I
                        FOR EACH ROW EXECUTE FUNCTION set_updated_at()', t, t);
    END LOOP;
END
$$;
-- +goose StatementEnd

-- ------------- Row-level security -------------

ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tenants
    USING (id = app_current_tenant()) WITH CHECK (id = app_current_tenant());

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'users', 'roles', 'role_permissions', 'user_roles', 'refresh_tokens', 'contacts',
        'conversations', 'messages', 'message_embeddings', 'conversation_summaries',
        'contact_memories', 'kb_modules', 'kb_versions', 'kb_chunks', 'kb_test_cases',
        'kb_suggestions', 'compliance_logs', 'agent_sessions', 'agent_traces', 'llm_usage',
        'meta_configs', 'meta_connections', 'meta_assets', 'meta_oauth_states',
        'llm_configs', 'embedding_configs'
    ] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('CREATE POLICY tenant_isolation ON %I
                        USING (tenant_id = app_current_tenant())
                        WITH CHECK (tenant_id = app_current_tenant())', t);
    END LOOP;
END
$$;
-- +goose StatementEnd

-- ------------- Grants for the service role -------------

GRANT USAGE ON SCHEMA public TO pharmachat_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO pharmachat_app;
REVOKE INSERT, UPDATE, DELETE ON permissions FROM pharmachat_app;
REVOKE UPDATE, DELETE ON agent_traces, compliance_logs FROM pharmachat_app;
-- Messages are immutable except for delivery bookkeeping; purging runs as the owner.
REVOKE UPDATE, DELETE ON messages FROM pharmachat_app;
GRANT UPDATE (external_message_id, metadata_json) ON messages TO pharmachat_app;
REVOKE ALL ON goose_db_version FROM pharmachat_app;
GRANT EXECUTE ON FUNCTION app_current_tenant() TO pharmachat_app;

-- ------------- Seed: permission catalogue -------------

INSERT INTO permissions (code, description) VALUES
    ('inbox.read',      'View unified inbox'),
    ('inbox.handoff',   'Take over and reply to conversations'),
    ('kb.read',         'Read Knowledge Base'),
    ('kb.write',        'Edit Knowledge Base'),
    ('kb.deploy',       'Deploy KB version'),
    ('agent.read',      'View agents'),
    ('agent.manage',    'Start / stop / configure agent'),
    ('ai_config.read',  'View AI configuration'),
    ('ai_config.write', 'Edit AI configuration'),
    ('admin.system',    'System administration'),
    ('admin.audit',     'View audit logs')
ON CONFLICT (code) DO NOTHING;

-- +goose Down

DROP TABLE IF EXISTS
    embedding_configs, llm_configs, meta_oauth_states, meta_assets, meta_connections, meta_configs,
    llm_usage, agent_traces, agent_sessions, compliance_logs, kb_suggestions, kb_test_cases,
    kb_chunks, kb_versions, kb_modules, contact_memories, conversation_summaries,
    message_embeddings, messages, conversations, contacts, refresh_tokens, user_roles,
    role_permissions, permissions, roles, users, tenants
CASCADE;
DROP FUNCTION IF EXISTS set_updated_at();
DROP FUNCTION IF EXISTS app_current_tenant();
