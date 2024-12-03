CREATE TABLE users(
  id serial PRIMARY KEY,
  username varchar(64) NOT NULL UNIQUE,
  password_hash varchar(64),
  created_at timestamp NOT NULL DEFAULT NOW()
);

CREATE TABLE balances(
  user_id integer NOT NULL REFERENCES users(id),
  currency varchar(64) NOT NULL,
  amount bigint NOT NULL,
  CONSTRAINT unique_user_currency UNIQUE (user_id, currency)
);

CREATE INDEX IF NOT EXISTS balances_user_id_idx ON balances(user_id);

CREATE INDEX IF NOT EXISTS balances_user_currency_idx ON balances(user_id, currency);

CREATE INDEX IF NOT EXISTS balances_currency_idx ON balances(currency);

CREATE TABLE permissions(
  id serial PRIMARY KEY,
  name varchar(32) NOT NULL UNIQUE
);

CREATE INDEX IF NOT EXISTS permissions_name_idx ON permissions(name);

CREATE TABLE user_permission(
  user_id integer NOT NULL REFERENCES users(id),
  permission_id integer NOT NULL REFERENCES permissions(id),
  CONSTRAINT unique_permissions UNIQUE (user_id, permission_id)
);

CREATE INDEX IF NOT EXISTS user_permission_user_id_idx ON user_permission(user_id);

CREATE INDEX IF NOT EXISTS user_permission_permission_id_idx ON
  user_permission(permission_id);

CREATE INDEX IF NOT EXISTS user_permission_user_id_permission_id_idx ON
  user_permission(user_id, permission_id);

CREATE TABLE recovery_code(
  user_id integer NOT NULL REFERENCES users(id),
  code varchar(12) NOT NULL,
  valid_until timestamp NOT NULL,
  CONSTRAINT unique_code UNIQUE (user_id, code)
);

CREATE INDEX IF NOT EXISTS recovery_code_user_id_idx ON recovery_code(user_id);

CREATE TABLE transaction_logs(
  id serial PRIMARY KEY,
  sender_id integer NOT NULL REFERENCES users(id),
  receiver_id integer NOT NULL REFERENCES users(id),
  initiator_id integer NOT NULL REFERENCES users(id),
  transaction_status integer REFERENCES error_description(code),
  sender_balance_after bigint NOT NULL,
  receiver_balance_after bigint NOT NULL,
  currency varchar(64) NOT NULL,
  amount bigint NOT NULL,
  created_at timestamp NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS transfer_logs_sender_id_idx ON transfer_logs(sender_id);

CREATE INDEX IF NOT EXISTS transfer_logs_receiver_id_idx ON transfer_logs(receiver_id);

CREATE INDEX IF NOT EXISTS transfer_logs_currency_idx ON transfer_logs(currency);

CREATE INDEX IF NOT EXISTS transfer_logs_created_at_idx ON transfer_logs(created_at);

CREATE INDEX IF NOT EXISTS transfer_logs_sender_receiver_idx ON
  transfer_logs(sender_id, receiver_id);

CREATE TABLE error_description(
  code integer PRIMARY KEY,
  description text NOT NULL
);

INSERT INTO error_description(code, description)
  VALUES (100, 'Transaction: Successful'),
(101, 'Transaction: Initiator is not the sender and does not have permission to manage funds'),
(102, 'Transaction: Sender does not have "send_funds" permission'),
(103, 'Transaction: Receiver does not have "receive_funds" permission'),
(104, 'Transaction: Insufficient funds'),
(105, 'Transaction: Amount less than or equal to zero');

INSERT INTO permissions(name)
  VALUES ('administrator'),
('manage_user_permissions'), --TODO
('manage_user_funds'),
('control_user_accounts'), --TODO
('print_money'), --TODO
('audit_funds'), --TODO
('receive_funds'),
('send_funds');

CREATE OR REPLACE FUNCTION log_transaction(
  sender_id integer,
  receiver_id integer,
  initiator_id integer,
  transaction_status integer,
  sender_balance_after bigint,
  receiver_balance_after bigint,
  currency varchar(64),
  amount bigint
)
  RETURNS void
  AS $$
BEGIN
  INSERT INTO transaction_logs(sender_id, receiver_id, initiator_id,
    transaction_status, sender_balance_after, receiver_balance_after, currency,
    amount)
    VALUES(sender_id, receiver_id, initiator_id, transaction_status,
      sender_balance_after, receiver_balance_after, currency, amount);
END;
$$
LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION check_transaction_permissions(
  initiator_id integer,
  sender_id integer,
  receiver_id integer
)
  RETURNS integer
  AS $$
BEGIN
  IF EXISTS(
    SELECT
      1
    FROM
      user_permission
      JOIN permissions ON permissions.id = user_permission.permission_id
    WHERE
      user_id = initiator_id
      AND permissions.name IN('manage_user_funds', 'administrator')) THEN
  RETURN 100;
  -- all good
ELSE
  IF initiator_id != sender_id THEN
    RETURN 101;
    -- initiator is not sender and does not have "manage_user_funds" permission
  END IF;
  IF NOT EXISTS(
    SELECT
      1
    FROM
      user_permission
      JOIN permissions ON permissions.id = user_permission.permission_id
    WHERE
      user_id = initiator_id
      AND permissions.name = 'send_funds') THEN
  RETURN 102;
  -- sender does not have "send_funds" permission
END IF;
  IF NOT EXISTS(
    SELECT
      1
    FROM
      user_permission
      JOIN permissions ON permissions.id = user_permission.permission_id
    WHERE
      user_id = receiver_id
      AND permissions.name = 'receive_funds') THEN
  RETURN 103;
  -- receiver does not have "receive_funds" permission
END IF;
END IF;
  RETURN 100;
END;
$$
LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION proceed_transaction(
  sender_id integer,
  receiver_id integer,
  initiator_id integer,
  currency_param varchar(64),
  amount_param bigint
)
  RETURNS integer
  AS $$
DECLARE
  sender_balance bigint;
  receiver_balance bigint;
  status_code integer;
BEGIN
  SELECT
    balances.amount INTO sender_balance
  FROM
    balances
  WHERE
    balances.user_id = sender_id
    AND balances.currency = currency_param
  FOR UPDATE;

  SELECT
    balances.amount INTO receiver_balance
  FROM
    balances
  WHERE
    balances.user_id = receiver_id
    AND balances.currency = currency_param
  FOR UPDATE;

  status_code := check_transaction_permissions(initiator_id, sender_id, receiver_id);
  IF status_code <> 100 THEN
    SELECT
      log_transaction(sender_id, receiver_id, initiator_id, status_code,
	sender_balance, receiver_balance, currency, amount);
    RETURN status_code;
    -- some permission error
  END IF;

  IF sender_balance < amount_param OR sender_balance IS NULL THEN
    SELECT
      log_transaction(sender_id, receiver_id, initiator_id, 4, sender_balance,
	receiver_balance, currency, amount);
    RETURN 104;
    -- insufficient funds
  END IF;

  IF amount_param <= 0 THEN
    SELECT
      log_transaction(sender_id, receiver_id, initiator_id, 5, sender_balance,
	receiver_balance, currency, amount);
    RETURN 105;
    -- transaction amount less than 0
  END IF;

  INSERT INTO balances(user_id, currency, amount)
    VALUES (receiver_id, currency_param, amount_param)
  ON CONFLICT (user_id, currency)
    DO UPDATE SET
      amount = balances.amount + EXCLUDED.amount;
  UPDATE
    balances
  SET
    amount = sender_balance - amount_param
  WHERE
    user_id = sender_id
    AND currency = currency_param;
  SELECT
    log_transaction(sender_id, receiver_id, initiator_id, 0, sender_balance,
      receiver_balance, currency, amount);
  RETURN 100;
  -- all good
END;
$$
LANGUAGE plpgsql;
