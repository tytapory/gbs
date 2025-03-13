
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

CREATE OR REPLACE FUNCTION register_user(username text, password_hash text)
RETURNS integer AS $$
DECLARE
new_user_id integer;
BEGIN
    IF EXISTS (SELECT 1 FROM users WHERE users.username = register_user.username) THEN
        RETURN 0;
END IF;

INSERT INTO users (username, password_hash)
VALUES (register_user.username, register_user.password_hash)
    RETURNING id INTO new_user_id;

RETURN new_user_id;
END;
$$ LANGUAGE plpgsql;
