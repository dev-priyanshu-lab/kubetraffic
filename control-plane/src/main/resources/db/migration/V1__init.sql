-- KubeTraffic control-plane schema, v1.

CREATE TABLE policy (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    namespace   VARCHAR(253) NOT NULL,
    name        VARCHAR(253) NOT NULL,
    spec        JSONB        NOT NULL,
    generation  BIGINT       NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_policy_ns_name UNIQUE (namespace, name)
);

CREATE TABLE config_version (
    id          BIGSERIAL   PRIMARY KEY,
    policy_id   UUID        NOT NULL REFERENCES policy(id) ON DELETE CASCADE,
    version     BIGINT      NOT NULL,
    spec        JSONB       NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_config_version UNIQUE (policy_id, version)
);

CREATE TABLE audit_log (
    id          BIGSERIAL    PRIMARY KEY,
    actor       VARCHAR(128) NOT NULL,
    action      VARCHAR(64)  NOT NULL,
    target      VARCHAR(512) NOT NULL,
    detail      JSONB,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_created_at ON audit_log (created_at DESC);
CREATE INDEX idx_audit_target     ON audit_log (target);
