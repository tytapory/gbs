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

CREATE TABLE transfer_logs(
  id serial PRIMARY KEY,
  sender_id integer NOT NULL REFERENCES users(id),
  receiver_id integer NOT NULL REFERENCES users(id),
  sender_balance_after bigint NOT NULL,
  receiver_balance_after bigint NOT NULL,
  currency varchar(64) NOT NULL,
  amount bigint NOT NULL,
  created_at timestamp NOT NULL DEFAULT NOW()
);

INSERT INTO permissions(name)
  VALUES ('administrator'),
('manage_user_permissions'),
('manage_user_funds'),
('control_user_accounts'),
('print_money'),
('audit_funds'),
('receive_funds'),
('send_funds');

CREATE OR REPLACE FUNCTION check_transaction_permissions(initiator_id integer, sender_id integer, receiver_id integer)
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
  RETURN 0;
  -- all good
ELSE
  IF initiator_id != sender_id THEN
    RETURN 1;
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
  RETURN 2;
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
  RETURN 3;
  -- receiver does not have "receive_funds" permission
END IF;
END IF;
  RETURN 0;
END;
$$
LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION proceed_transaction(sender_id integer, receiver_id integer, initiator_id integer, currency_param varchar(64), amount_param bigint)
  RETURNS integer
  AS $$
DECLARE
  sender_balance bigint;
  err_code integer;
BEGIN
  err_code := check_transaction_permissions(initiator_id, sender_id, receiver_id);
  IF err_code <> 0 THEN
    RETURN err_code;
    -- some permission error
  END IF;

  SELECT
    balances.amount INTO sender_balance
  FROM
    balances
  WHERE
    balances.user_id = sender_id
    AND balances.currency = currency_param
  FOR UPDATE;

  IF sender_balance < amount_param OR sender_balance IS NULL THEN
    RETURN 4;
    -- insufficient funds
  END IF;

  IF amount_param <= 0 THEN 
    RETURN 5;
    -- transaction amount less than 0
  END IF;

  PERFORM
    1
  FROM
    balances
  WHERE
    user_id = receiver_id
    AND balances.currency = currency_param
  FOR UPDATE;

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
  RETURN 0;
  -- all good
END;
$$
LANGUAGE plpgsql;

