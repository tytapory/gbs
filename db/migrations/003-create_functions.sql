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

CREATE OR REPLACE FUNCTION raise_error(code integer)
RETURNS void AS $$
DECLARE
error_text text;
BEGIN
SELECT description INTO error_text FROM error_description WHERE error_description.code = raise_error.code;
IF error_text IS NULL THEN
        error_text := 'Unknown error';
END IF;
    RAISE EXCEPTION '%', error_text USING ERRCODE = 'P0001';
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION check_transaction_permissions(
  initiator_id integer,
  sender_id integer,
  receiver_id integer
)
  RETURNS void AS $$
BEGIN
  IF EXISTS(
    SELECT 1 FROM user_permission
      JOIN permissions ON permissions.id = user_permission.permission_id
    WHERE user_id = initiator_id
      AND permissions.name IN ('manage_user_funds', 'administrator')) THEN
    RETURN;
END IF;

  IF initiator_id != sender_id THEN
    PERFORM raise_error(104);
END IF;

  IF NOT EXISTS(
    SELECT 1 FROM user_permission
      JOIN permissions ON permissions.id = user_permission.permission_id
    WHERE user_id = initiator_id
      AND permissions.name = 'send_funds') THEN
    PERFORM raise_error(105);
END IF;

  IF NOT EXISTS(
    SELECT 1 FROM user_permission
      JOIN permissions ON permissions.id = user_permission.permission_id
    WHERE user_id = receiver_id
      AND permissions.name = 'receive_funds') THEN
    PERFORM raise_error(106);
END IF;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION proceed_transaction(
  sender_id integer,
  receiver_id integer,
  initiator_id integer,
  currency_param varchar(64),
  amount_param bigint,
  fee integer
)
  RETURNS void AS $$
DECLARE
sender_balance bigint;
  receiver_balance bigint;
  commission_amount bigint;
BEGIN
  IF NOT EXISTS (SELECT 1 FROM users WHERE id = sender_id) THEN
    PERFORM raise_error(101);
END IF;

  IF NOT EXISTS (SELECT 1 FROM users WHERE id = receiver_id) THEN
    PERFORM raise_error(102);
END IF;

  IF NOT EXISTS (SELECT 1 FROM users WHERE id = initiator_id) THEN
    PERFORM raise_error(103);
END IF;

SELECT amount INTO sender_balance FROM balances WHERE user_id = sender_id AND currency = currency_param FOR UPDATE;
SELECT amount INTO receiver_balance FROM balances WHERE user_id = receiver_id AND currency = currency_param FOR UPDATE;

PERFORM check_transaction_permissions(initiator_id, sender_id, receiver_id);

  IF sender_balance < amount_param OR sender_balance IS NULL THEN
    PERFORM raise_error(107);
END IF;

  IF amount_param <= 0 THEN
    PERFORM raise_error(108);
END IF;

  commission_amount := (amount_param * fee + 9999) / 10000;

INSERT INTO balances(user_id, currency, amount)
VALUES (receiver_id, currency_param, amount_param - commission_amount)
    ON CONFLICT (user_id, currency)
    DO UPDATE SET amount = balances.amount + EXCLUDED.amount;

INSERT INTO balances(user_id, currency, amount)
VALUES (2, currency_param, commission_amount)
    ON CONFLICT (user_id, currency)
    DO UPDATE SET amount = balances.amount + EXCLUDED.amount;

UPDATE balances
SET amount = sender_balance - amount_param
WHERE user_id = sender_id AND currency = currency_param;

SELECT amount INTO receiver_balance FROM balances WHERE user_id = receiver_id AND currency = currency_param;
SELECT amount INTO sender_balance FROM balances WHERE user_id = sender_id AND currency = currency_param;

PERFORM log_transaction(sender_id, receiver_id, initiator_id, 100,
                          sender_balance, receiver_balance,
                          currency_param, amount_param, commission_amount);
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION print_money(
  receiver_id integer,
  initiator_id integer,
  currency_param varchar(64),
  amount_param bigint
)
  RETURNS void AS $$
DECLARE
receiver_balance bigint;
BEGIN
  IF NOT EXISTS (SELECT 1 FROM users WHERE id = receiver_id) THEN
    PERFORM raise_error(201);
END IF;

  IF NOT EXISTS (SELECT 1 FROM users WHERE id = initiator_id) THEN
    PERFORM raise_error(202);
END IF;

  IF NOT EXISTS (
    SELECT 1 FROM user_permission
      JOIN permissions ON permissions.id = user_permission.permission_id
    WHERE user_id = initiator_id
      AND (permissions.name = 'print_money' OR permissions.name = 'administrator')) THEN
    PERFORM raise_error(203);
END IF;

  IF amount_param <= 0 THEN
    PERFORM raise_error(204);
END IF;

INSERT INTO balances(user_id, currency, amount)
VALUES (receiver_id, currency_param, amount_param)
    ON CONFLICT (user_id, currency)
    DO UPDATE SET amount = balances.amount + EXCLUDED.amount;

SELECT amount INTO receiver_balance FROM balances WHERE user_id = receiver_id;

PERFORM log_print_money(receiver_id, initiator_id, 200, receiver_balance,
                          currency_param, amount_param);
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION get_balances(initiator_id_param integer, user_id_param integer)
RETURNS TABLE(currency varchar(64), amount bigint) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM user_permission
        WHERE initiator_id_param = user_id AND
        (permission_id = 1 OR permission_id = 6)
    ) AND user_id_param != initiator_id_param THEN PERFORM raise_error(301);
    END IF;

    RETURN QUERY
    SELECT balances.currency, balances.amount FROM balances WHERE user_id = user_id_param;
END;
$$ LANGUAGE plpgsql;

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

CREATE OR REPLACE FUNCTION get_amount_of_user_transactions(initiator_id_param integer, user_id_param integer)
RETURNS integer AS $$
DECLARE
transaction_count INTEGER;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM user_permission
        WHERE initiator_id_param = user_id AND
        (permission_id = 1 OR permission_id = 6)
    ) AND user_id_param != initiator_id_param THEN PERFORM raise_error(301);
    END IF;

    SELECT COUNT(*) INTO transaction_count
    FROM transaction_logs
    WHERE (sender_id = user_id_param OR receiver_id = user_id_param)
    AND transaction_status = 100;

    RETURN transaction_count;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION get_transaction_history(initiator_id_param integer, user_id_param integer, limit_param integer, offset_param integer)
RETURNS TABLE(sender_id integer, receiver_id integer, initiator_id integer, currency varchar(64), amount bigint, fee bigint, created_at timestamp) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM user_permission
        WHERE initiator_id_param = user_id AND
        (permission_id = 1 OR permission_id = 6)
    ) AND user_id_param != initiator_id_param THEN PERFORM raise_error(301);
    END IF;

    RETURN QUERY
    SELECT transaction_logs.sender_id, transaction_logs.receiver_id, transaction_logs.initiator_id, transaction_logs.currency, transaction_logs.amount, transaction_logs.fee, transaction_logs.created_at
    FROM transaction_logs
    WHERE (transaction_logs.sender_id = user_id_param
    OR transaction_logs.receiver_id = user_id_param)
    AND transaction_status = 100
    ORDER BY transaction_logs.created_at DESC
    OFFSET offset_param LIMIT limit_param;
END;
$$ LANGUAGE plpgsql;