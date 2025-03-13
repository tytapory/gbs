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

CREATE TABLE recovery_code(
  user_id integer NOT NULL REFERENCES users(id),
  code varchar(12) NOT NULL,
  valid_until timestamp NOT NULL,
  CONSTRAINT unique_code UNIQUE (user_id, code)
);

CREATE TABLE error_description(
  code integer NOT NULL UNIQUE,
  description text NOT NULL
);

CREATE TABLE transaction_logs(
  id serial PRIMARY KEY,
  sender_id integer NOT NULL REFERENCES users(id),
  receiver_id integer NOT NULL REFERENCES users(id),
  initiator_id integer NOT NULL REFERENCES users(id),
  transaction_status integer REFERENCES error_description(code),
  sender_balance_after bigint DEFAULT 0,
  receiver_balance_after bigint DEFAULT 0,
  currency varchar(64) NOT NULL,
  amount bigint NOT NULL,
  fee bigint NOT NULL,
  created_at timestamp NOT NULL DEFAULT NOW()
);

CREATE TABLE print_money_logs(
  id serial PRIMARY KEY,
  receiver_id integer NOT NULL REFERENCES users(id),
  initiator_id integer NOT NULL REFERENCES users(id),
  print_status integer REFERENCES error_description(code),
  receiver_balance_after bigint DEFAULT 0,
  currency varchar(64) NOT NULL,
  amount bigint NOT NULL,
  created_at timestamp NOT NULL DEFAULT NOW()
);