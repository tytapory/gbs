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

CREATE TABLE error_description(
  code integer NOT NULL UNIQUE,
  description text NOT NULL
);

INSERT INTO error_description(code, description)
  VALUES (100, 'Transaction: Successful'),
(101, 'Transaction: Sender does not exist'),
(102, 'Transaction: Receiver does not exist'),
(103, 'Transaction: Initiator does not exist'),
(104, 'Transaction: Initiator is not the sender and does not have permission to manage funds'),
(105, 'Transaction: Sender does not have "send_funds" permission'),
(106, 'Transaction: Receiver does not have "receive_funds" permission'),
(107, 'Transaction: Insufficient funds'),
(108, 'Transaction: Amount less than or equal to zero'),
(200, 'Print money: Successful'),
(201, 'Print money: Receiver does not exist'),
(202, 'Print money: Initiator does not exist'),
(203, 'Print money: Initiator does not have permission to print money'),
(203, 'Print money: Cant print values <= 0');

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

CREATE INDEX IF NOT EXISTS transaction_logs_sender_id_idx ON
  transaction_logs(sender_id);

CREATE INDEX IF NOT EXISTS transaction_logs_receiver_id_idx ON
  transaction_logs(receiver_id);

CREATE INDEX IF NOT EXISTS transaction_logs_sender_id_idx ON
  transaction_logs(initiator_id);

CREATE INDEX IF NOT EXISTS transaction_logs_currency_idx ON transaction_logs(currency);

CREATE INDEX IF NOT EXISTS transaction_logs_created_at_idx ON
  transaction_logs(created_at);

CREATE INDEX IF NOT EXISTS transaction_logs_sender_receiver_idx ON
  transaction_logs(sender_id, receiver_id);

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

CREATE INDEX IF NOT EXISTS print_money_logs_sender_id_idx ON
  print_money_logs(initiator_id);

CREATE INDEX IF NOT EXISTS print_money_logs_receiver_id_idx ON
  print_money_logs(receiver_id);

CREATE INDEX IF NOT EXISTS print_money_logs_currency_idx ON print_money_logs(currency);

CREATE INDEX IF NOT EXISTS print_money_logs_created_at_idx ON
  print_money_logs(created_at);

CREATE INDEX IF NOT EXISTS print_money_logs_initiator_receiver_idx ON
  print_money_logs(initiator_id, receiver_id);

INSERT INTO permissions(name)
  VALUES ('administrator'),
('manage_user_permissions'), --TODO
('manage_user_funds'),
('control_user_accounts'), --TODO
('print_money'),
('audit_funds'), --TODO
('receive_funds'),
('send_funds');

INSERT INTO users(id, username)
  VALUES (1, 'adm'),
(2, 'fees'),
(3, 'registration'),
(4, 'money_printer');

INSERT INTO user_permission(user_id, permission_id)
  VALUES (1, 1),
(3, 4),
(4, 5);

CREATE OR REPLACE FUNCTION log_transaction(
  sender_id integer,
  receiver_id integer,
  initiator_id integer,
  transaction_status integer,
  sender_balance_after bigint,
  receiver_balance_after bigint,
  currency varchar(64),
  amount bigint,
  fee bigint
)
  RETURNS void
  AS $$
BEGIN
  INSERT INTO transaction_logs(sender_id, receiver_id, initiator_id,
    transaction_status, sender_balance_after, receiver_balance_after, currency,
    amount, fee)
    VALUES(sender_id, receiver_id, initiator_id, transaction_status,
      sender_balance_after, receiver_balance_after, currency, amount, fee);
END;
$$
LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION log_print_money(
  receiver_id integer,
  initiator_id integer,
  print_status integer,
  receiver_balance_after bigint,
  currency varchar(64),
  amount bigint
)
  RETURNS void
  AS $$
BEGIN
  INSERT INTO print_money_logs(receiver_id, initiator_id, print_status,
    receiver_balance_after, currency, amount)
    VALUES(receiver_id, initiator_id, print_status, receiver_balance_after,
      currency, amount);
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
    RETURN 104;
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
  RETURN 105;
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
  RETURN 106;
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
  amount_param bigint,
  fee integer
)
  RETURNS integer
  AS $$
DECLARE
  sender_balance bigint;
  receiver_balance bigint;
  status_code integer;
  commission_amount bigint;
BEGIN
  IF NOT EXISTS (
    SELECT
      1
    FROM
      users
    WHERE
      id = sender_id) THEN
  RETURN 101;
END IF;
  IF NOT EXISTS (
    SELECT
      1
    FROM
      users
    WHERE
      id = receiver_id) THEN
  RETURN 102;
END IF;
  IF NOT EXISTS (
    SELECT
      1
    FROM
      users
    WHERE
      id = initiator_id) THEN
  RETURN 103;
END IF;
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
    PERFORM
      log_transaction(sender_id, receiver_id, initiator_id, status_code,
	sender_balance, receiver_balance, currency_param, amount_param, 0);
    RETURN status_code;
    -- some permission error
  END IF;

  IF sender_balance < amount_param OR sender_balance IS NULL THEN
    PERFORM
      log_transaction(sender_id, receiver_id, initiator_id, 107,
	sender_balance, receiver_balance, currency_param, amount_param, 0);
    RETURN 107;
    -- insufficient funds
  END IF;

  IF amount_param <= 0 THEN
    PERFORM
      log_transaction(sender_id, receiver_id, initiator_id, 108,
	sender_balance, receiver_balance, currency_param, amount_param, 0);
    RETURN 108;
    -- transaction amount less than 0
  END IF;

  commission_amount :=(amount_param * fee + 9999) / 10000;
INSERT INTO balances(user_id, currency, amount)
  VALUES (receiver_id, currency_param, amount_param - commission_amount)
ON CONFLICT (user_id, currency)
  DO UPDATE SET
    amount = balances.amount + EXCLUDED.amount;

  PERFORM
    amount
  FROM
    balances
  WHERE
    user_id = 2
    AND currency = currency_param
  FOR UPDATE;

INSERT INTO balances(user_id, currency, amount)
  VALUES (2, currency_param, commission_amount)
ON CONFLICT (user_id, currency)
  DO UPDATE SET
    amount = commission_amount + EXCLUDED.amount;

  UPDATE
    balances
  SET
    amount = sender_balance - amount_param
  WHERE
    user_id = sender_id
    AND currency = currency_param;

  SELECT
    balances.amount INTO receiver_balance
  FROM
    balances
  WHERE
    balances.user_id = receiver_id
    AND balances.currency = currency_param;
  SELECT
    balances.amount INTO sender_balance
  FROM
    balances
  WHERE
    balances.user_id = sender_id
    AND balances.currency = currency_param;

  PERFORM
    log_transaction(sender_id, receiver_id, initiator_id, 100, sender_balance,
      receiver_balance, currency_param, amount_param, commission_amount);
  RETURN 100;
  -- all good
END;
$$
LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION print_money(
  receiver_id integer,
  initiator_id integer,
  currency_param varchar(64),
  amount_param bigint
)
  RETURNS integer
  AS $$
DECLARE
  receiver_balance bigint;
BEGIN
  IF NOT EXISTS (
    SELECT
      1
    FROM
      users
    WHERE
      id = receiver_id) THEN
  PERFORM
    log_print_money(receiver_id, initiator_id, 201, 0, currency_param, amount_param);
  RETURN 201;
END IF;
  SELECT
    amount INTO receiver_balance
  FROM
    balances
  WHERE
    receiver_id = balances.user_id
  FOR UPDATE;

  IF NOT EXISTS (
    SELECT
      1
    FROM
      users
    WHERE
      id = initiator_id) THEN
  PERFORM
    log_print_money(receiver_id, initiator_id, 202, receiver_balance,
      currency_param, amount_param);
  RETURN 202;
END IF;

  IF NOT EXISTS (
    SELECT
      *
    FROM
      user_permission
      JOIN permissions ON permissions.id = user_permission.permission_id
    WHERE
      user_id = initiator_id
      AND (permissions.name = 'print_money'
        OR permissions.name = 'administrator')) THEN
  PERFORM
    log_print_money(receiver_id, initiator_id, 203, receiver_balance,
      currency_param, amount_param);
  RETURN 203;
END IF;
  IF amount_param <= 0 THEN
    PERFORM
      log_print_money(receiver_id, initiator_id, 204, receiver_balance,
	currency_param, amount_param);

    RETURN 204;
  END IF;

INSERT INTO balances(user_id, currency, amount)
  VALUES (receiver_id, currency_param, amount_param)
ON CONFLICT (user_id, currency)
  DO UPDATE SET
    amount = balances.amount + EXCLUDED.amount;
  SELECT
    amount INTO receiver_balance
  FROM
    balances
  WHERE
    receiver_id = balances.user_id;
  PERFORM
    log_print_money(receiver_id, initiator_id, 200, receiver_balance, currency_param, amount_param);
  RETURN 200;
END;
$$
LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION register_user(username text, password_hash char(60))
RETURNS integer AS $$
DECLARE
    new_user_id integer;
BEGIN
    IF EXISTS (SELECT 1 FROM users WHERE users.username = register_user.username) THEN
        RETURN NULL;
    END IF;

    INSERT INTO users (username, user_hash)
    VALUES (register_user.username, register_user.password_hash)
    RETURNING id INTO new_user_id;

    RETURN new_user_id;
END;
$$ LANGUAGE plpgsql;

