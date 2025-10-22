CREATE TABLE users(
  id serial PRIMARY KEY,
  username varchar(64) NOT NULL UNIQUE,
  password_hash char(60),
  created_at timestamp NOT NULL DEFAULT NOW()
);

CREATE TABLE balances(
  user_id integer NOT NULL REFERENCES users(id),
  currency varchar(64) NOT NULL,
  amount bigint NOT NULL,
  CONSTRAINT unique_user_currency UNIQUE (user_id, currency)
);

CREATE TABLE permissions(
  id serial PRIMARY KEY,
  name varchar(32) NOT NULL UNIQUE
);

CREATE TABLE user_permission(
  user_id integer NOT NULL REFERENCES users(id),
  permission_id integer NOT NULL REFERENCES permissions(id),
  CONSTRAINT unique_permissions UNIQUE (user_id, permission_id)
);

CREATE TABLE transaction_logs(
  id serial PRIMARY KEY,
  sender_id integer REFERENCES users(id),
  receiver_id integer NOT NULL REFERENCES users(id),
  initiator_id integer NOT NULL REFERENCES users(id),
  sender_balance_after bigint DEFAULT NULL,
  receiver_balance_after bigint NOT NULL,
  currency varchar(64) NOT NULL,
  amount bigint NOT NULL,
  fee bigint NOT NULL,
  created_at timestamp NOT NULL DEFAULT NOW()
);

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE refresh_tokens (
    user_id INTEGER NOT NULL,
    token UUID NOT NULL DEFAULT gen_random_uuid(),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked BOOLEAN NOT NULL DEFAULT false,
    CONSTRAINT fk_user FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);