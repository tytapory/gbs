CREATE TABLE users (
    id UUID PRIMARY KEY,
    username varchar(64) NOT NULL UNIQUE,
    password_hash char(60),
    created_at timestamp NOT NULL DEFAULT NOW()
);

CREATE TABLE permissions (
    id serial PRIMARY KEY,
    name varchar(32) NOT NULL UNIQUE
);

CREATE TABLE balances (
    user_id UUID NOT NULL REFERENCES users(id),
    currency varchar(64) NOT NULL,
    amount bigint NOT NULL CHECK (amount >= 0),
    CONSTRAINT unique_user_currency UNIQUE (user_id, currency)
);

CREATE TABLE user_permission (
    user_id UUID NOT NULL REFERENCES users(id),
    permission_id integer NOT NULL REFERENCES permissions(id),
    CONSTRAINT unique_permissions UNIQUE (user_id, permission_id)
);

CREATE TABLE transaction_logs (
    id UUID PRIMARY KEY,
    sender_id UUID REFERENCES users(id),
    receiver_id UUID NOT NULL REFERENCES users(id),
    initiator_id UUID NOT NULL REFERENCES users(id),
    sender_balance_after bigint DEFAULT NULL CHECK (sender_balance_after IS NULL OR sender_balance_after >= 0),
    receiver_balance_after bigint NOT NULL CHECK (receiver_balance_after >= 0),
    currency varchar(64) NOT NULL,
    amount bigint NOT NULL CHECK (amount > 0),
    fee bigint DEFAULT NULL CHECK (fee IS NULL OR fee >= 0),
    created_at timestamp NOT NULL DEFAULT NOW()
);

CREATE TABLE refresh_tokens (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token UUID PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked BOOLEAN NOT NULL DEFAULT false
);
