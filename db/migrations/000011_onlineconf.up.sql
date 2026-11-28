BEGIN;

CREATE SCHEMA onlineconf;

CREATE TABLE onlineconf.rollout_policies (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    stages SMALLINT[] NOT NULL,
    min_interval INTERVAL NOT NULL DEFAULT INTERVAL '1 minute'
        CHECK (min_interval >= INTERVAL '0 seconds'),
    requires_approval BOOLEAN NOT NULL DEFAULT TRUE,
    created_by BIGINT REFERENCES public."user" (id),
    CHECK (
        array_ndims(stages) = 1
        AND array_lower(stages, 1) = 1
        AND cardinality(stages) > 0
        AND array_position(stages, NULL) IS NULL
        AND 0 < ALL(stages)
        AND 100 >= ALL(stages)
    )
);

CREATE TABLE onlineconf.configs (
    id BIGSERIAL PRIMARY KEY,
    scope TEXT NOT NULL CHECK (scope IN ('frontend', 'gateway', 'auth', 'profile', 'matching', 'chat')),
    name TEXT NOT NULL CHECK (name ~ '^[a-z0-9_]{1,64}$'),
    kind TEXT NOT NULL CHECK (kind IN ('flag', 'config')),
    value_type TEXT NOT NULL CHECK (value_type IN ('bool', 'number', 'string', 'array', 'json')),
    value JSONB NOT NULL,
    rollout_percent SMALLINT,
    policy_id BIGINT REFERENCES onlineconf.rollout_policies (id),
    stage_index INTEGER CHECK (stage_index >= 0),
    stage_changed_at TIMESTAMPTZ,
    client_visible BOOLEAN NOT NULL DEFAULT FALSE,
    description TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by BIGINT REFERENCES public."user" (id),
    deleted_at TIMESTAMPTZ,
    CHECK (
        (value_type = 'bool' AND jsonb_typeof(value) = 'boolean')
        OR (value_type = 'number' AND jsonb_typeof(value) = 'number')
        OR (value_type = 'string' AND jsonb_typeof(value) = 'string')
        OR (value_type = 'array' AND jsonb_typeof(value) = 'array')
        OR value_type = 'json'
    ),
    CHECK (
        (kind = 'flag' AND value_type = 'bool' AND rollout_percent IS NOT NULL
            AND rollout_percent BETWEEN 0 AND 100)
        OR (kind = 'config' AND rollout_percent IS NULL AND policy_id IS NULL
            AND stage_index IS NULL AND stage_changed_at IS NULL)
    ),
    CHECK (
        (policy_id IS NULL AND stage_index IS NULL AND stage_changed_at IS NULL)
        OR (policy_id IS NOT NULL AND stage_index IS NOT NULL AND stage_changed_at IS NOT NULL)
    )
);

CREATE UNIQUE INDEX configs_active_scope_name_idx
    ON onlineconf.configs (scope, name)
    WHERE deleted_at IS NULL;

CREATE TABLE onlineconf.flag_overrides (
    config_id BIGINT NOT NULL REFERENCES onlineconf.configs (id),
    user_id BIGINT NOT NULL REFERENCES public."user" (id),
    enabled BOOLEAN NOT NULL,
    created_by BIGINT NOT NULL REFERENCES public."user" (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (config_id, user_id)
);

CREATE TABLE onlineconf.history (
    id BIGSERIAL PRIMARY KEY,
    config_id BIGINT REFERENCES onlineconf.configs (id),
    entity TEXT NOT NULL CHECK (entity IN ('config', 'flag', 'admin', 'rollout_policy')),
    action TEXT NOT NULL CHECK (action IN ('create', 'update', 'delete', 'grant', 'revoke')),
    "before" JSONB,
    "after" JSONB,
    actor_id BIGINT REFERENCES public."user" (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX history_config_created_idx
    ON onlineconf.history (config_id, created_at DESC, id DESC);

CREATE TABLE onlineconf.admins (
    user_id BIGINT PRIMARY KEY REFERENCES public."user" (id),
    granted_by BIGINT REFERENCES public."user" (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE onlineconf.state (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0)
);

INSERT INTO onlineconf.state (id) VALUES (1);

WITH policy AS (
    INSERT INTO onlineconf.rollout_policies (name, stages, min_interval, requires_approval)
    VALUES ('Стандарт 1→10→25→50→100', ARRAY[1, 10, 25, 50, 100]::SMALLINT[], INTERVAL '1 minute', TRUE)
    RETURNING *
)
INSERT INTO onlineconf.history (entity, action, "after")
SELECT 'rollout_policy', 'create', to_jsonb(policy) FROM policy;

WITH configs AS (
    INSERT INTO onlineconf.configs
        (scope, name, kind, value_type, value, rollout_percent, client_visible, description)
    VALUES
        ('profile', 'location_map_enabled', 'flag', 'bool', 'true'::JSONB, 100, TRUE,
            'Интерактивная карта и поиск по городу'),
        ('profile', 'max_photos', 'config', 'number', '6'::JSONB, NULL, TRUE,
            'Максимальное количество фотографий профиля')
    RETURNING *
)
INSERT INTO onlineconf.history (config_id, entity, action, "after")
SELECT id, kind, 'create', to_jsonb(configs) FROM configs;

COMMIT;
