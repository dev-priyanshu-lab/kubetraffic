-- Canary progression state, one row per (policy, path).
CREATE TABLE canary_state (
    id             BIGSERIAL    PRIMARY KEY,
    policy_id      UUID         NOT NULL REFERENCES policy(id) ON DELETE CASCADE,
    path           VARCHAR(512) NOT NULL,
    stable_version VARCHAR(63)  NOT NULL,
    canary_version VARCHAR(63)  NOT NULL,
    steps          JSONB        NOT NULL,
    step_index     INT          NOT NULL DEFAULT 0,
    status         VARCHAR(32)  NOT NULL,
    started_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_canary_policy_path UNIQUE (policy_id, path)
);

CREATE INDEX idx_canary_policy ON canary_state (policy_id);
