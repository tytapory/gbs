CREATE OR REPLACE FUNCTION log_transaction(
  sender_id_param integer,
  receiver_id_param integer,
  initiator_id_param integer,
  transaction_status_param integer,
  sender_balance_after_param bigint,
  receiver_balance_after_param bigint,
  currency_param varchar(64),
  amount_param bigint,
  fee_param bigint
)
  RETURNS void
  AS $$
BEGIN
INSERT INTO transaction_logs(
    sender_id, receiver_id, initiator_id,
    transaction_status, sender_balance_after, receiver_balance_after, currency,
    amount, fee
)
VALUES(
          sender_id_param, receiver_id_param, initiator_id_param, transaction_status_param,
          sender_balance_after_param, receiver_balance_after_param, currency_param, amount_param, fee_param
      );
END;
$$
LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION log_print_money(
  receiver_id_param integer,
  initiator_id_param integer,
  print_status_param integer,
  receiver_balance_after_param bigint,
  currency_param varchar(64),
  amount_param bigint
)
  RETURNS void
  AS $$
BEGIN
INSERT INTO print_money_logs(
    receiver_id, initiator_id, print_status,
    receiver_balance_after, currency, amount
)
VALUES(
          receiver_id_param, initiator_id_param, print_status_param,
          receiver_balance_after_param, currency_param, amount_param
      );
END;
$$
LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION raise_error(
  code_param integer
)
  RETURNS void AS $$
DECLARE
error_text text;
BEGIN
SELECT description
INTO error_text
FROM error_description
WHERE error_description.code = code_param;
IF error_text IS NULL THEN
    error_text := 'Unknown error';
END IF;
  RAISE EXCEPTION '%', error_text USING ERRCODE = 'P0001';
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION check_transaction_permissions(
  initiator_id_param integer,
  sender_id_param integer,
  receiver_id_param integer
)
  RETURNS void AS $$
BEGIN
  IF EXISTS(
      SELECT 1 FROM user_permission
        JOIN permissions ON permissions.id = user_permission.permission_id
       WHERE user_id = initiator_id_param
         AND permissions.name IN ('manage_user_funds', 'administrator')
  ) THEN
    RETURN;
END IF;

  IF initiator_id_param != sender_id_param THEN
    PERFORM raise_error(104);
END IF;

  IF NOT EXISTS(
      SELECT 1 FROM user_permission
        JOIN permissions ON permissions.id = user_permission.permission_id
       WHERE user_id = initiator_id_param
         AND permissions.name = 'send_funds'
  ) THEN
    PERFORM raise_error(105);
END IF;

  IF NOT EXISTS(
      SELECT 1 FROM user_permission
        JOIN permissions ON permissions.id = user_permission.permission_id
       WHERE user_id = receiver_id_param
         AND permissions.name = 'receive_funds'
  ) THEN
    PERFORM raise_error(106);
END IF;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION proceed_transaction(
  sender_id_param integer,
  receiver_id_param integer,
  initiator_id_param integer,
  currency_param varchar(64),
  amount_param bigint,
  fee_param integer
)
  RETURNS void AS $$
DECLARE
sender_balance bigint;
  receiver_balance bigint;
  commission_amount bigint;
BEGIN
  IF NOT EXISTS (SELECT 1 FROM users WHERE id = sender_id_param) THEN
    PERFORM raise_error(101);
END IF;

  IF NOT EXISTS (SELECT 1 FROM users WHERE id = receiver_id_param) THEN
    PERFORM raise_error(102);
END IF;

  IF NOT EXISTS (SELECT 1 FROM users WHERE id = initiator_id_param) THEN
    PERFORM raise_error(103);
END IF;

SELECT amount
INTO sender_balance
FROM balances
WHERE user_id = sender_id_param AND currency = currency_param
    FOR UPDATE;

SELECT amount
INTO receiver_balance
FROM balances
WHERE user_id = receiver_id_param AND currency = currency_param
    FOR UPDATE;

PERFORM check_transaction_permissions(initiator_id_param, sender_id_param, receiver_id_param);

  IF sender_balance < amount_param OR sender_balance IS NULL THEN
    PERFORM raise_error(107);
END IF;

  IF amount_param <= 0 THEN
    PERFORM raise_error(108);
END IF;

  commission_amount := (amount_param * fee_param + 9999) / 10000;

INSERT INTO balances(user_id, currency, amount)
VALUES (
           receiver_id_param, currency_param, amount_param - commission_amount
       )
    ON CONFLICT (user_id, currency)
    DO UPDATE SET amount = balances.amount + EXCLUDED.amount;

INSERT INTO balances(user_id, currency, amount)
VALUES (
           2, currency_param, commission_amount
       )
    ON CONFLICT (user_id, currency)
    DO UPDATE SET amount = balances.amount + EXCLUDED.amount;

UPDATE balances
SET amount = sender_balance - amount_param
WHERE user_id = sender_id_param AND currency = currency_param;

SELECT amount
INTO receiver_balance
FROM balances
WHERE user_id = receiver_id_param AND currency = currency_param;

SELECT amount
INTO sender_balance
FROM balances
WHERE user_id = sender_id_param AND currency = currency_param;

PERFORM log_transaction(
      sender_id_param, receiver_id_param, initiator_id_param, 100,
      sender_balance, receiver_balance,
      currency_param, amount_param, commission_amount
  );
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION print_money(
  receiver_id_param integer,
  initiator_id_param integer,
  currency_param varchar(64),
  amount_param bigint
)
  RETURNS void AS $$
DECLARE
receiver_balance bigint;
BEGIN
  IF NOT EXISTS (SELECT 1 FROM users WHERE id = receiver_id_param) THEN
    PERFORM raise_error(201);
END IF;

  IF NOT EXISTS (SELECT 1 FROM users WHERE id = initiator_id_param) THEN
    PERFORM raise_error(202);
END IF;

  IF NOT EXISTS (
      SELECT 1 FROM user_permission
      JOIN permissions ON permissions.id = user_permission.permission_id
     WHERE user_id = initiator_id_param
       AND (permissions.name = 'print_money' OR permissions.name = 'administrator')
  ) THEN
    PERFORM raise_error(203);
END IF;

  IF amount_param <= 0 THEN
    PERFORM raise_error(204);
END IF;

INSERT INTO balances(user_id, currency, amount)
VALUES (receiver_id_param, currency_param, amount_param)
    ON CONFLICT (user_id, currency)
    DO UPDATE SET amount = balances.amount + EXCLUDED.amount;

SELECT amount
INTO receiver_balance
FROM balances
WHERE user_id = receiver_id_param;

PERFORM log_print_money(
      receiver_id_param, initiator_id_param, 200, receiver_balance,
      currency_param, amount_param
  );
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION get_balances(
    initiator_id_param integer,
    user_id_param integer
)
    RETURNS TABLE(currency varchar(64), amount bigint) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM user_permission
        WHERE initiator_id_param = user_id AND
              (permission_id = 1 OR permission_id = 6)
    ) AND user_id_param != initiator_id_param THEN
      PERFORM raise_error(301);
END IF;

RETURN QUERY
SELECT balances.currency, balances.amount
FROM balances
WHERE user_id = user_id_param;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION register_user(
  username_param text,
  password_hash_param text
)
RETURNS integer AS $$
DECLARE
new_user_id integer;
BEGIN
  IF EXISTS (
      SELECT 1 FROM users WHERE users.username = username_param
  ) THEN
    RETURN 0;
END IF;

INSERT INTO users (username, password_hash)
VALUES (username_param, password_hash_param)
    RETURNING id INTO new_user_id;

RETURN new_user_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION get_amount_of_user_transactions(
  initiator_id_param integer,
  user_id_param integer
)
RETURNS integer AS $$
DECLARE
transaction_count INTEGER;
BEGIN
  IF NOT EXISTS (
      SELECT 1 FROM user_permission
      WHERE initiator_id_param = user_id AND
            (permission_id = 1 OR permission_id = 6)
  ) AND user_id_param != initiator_id_param THEN
    PERFORM raise_error(301);
END IF;

SELECT
    (
        SELECT COUNT(*)
        FROM transaction_logs
        WHERE (sender_id = user_id_param OR receiver_id = user_id_param)
          AND transaction_status = 100
    )
        +
    (
        SELECT COUNT(*)
        FROM print_money_logs
        WHERE (initiator_id = user_id_param OR receiver_id = user_id_param)
          AND print_status = 200
    )
INTO transaction_count;

RETURN transaction_count;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION get_transaction_history(
  initiator_id_param INTEGER,
  user_id_param INTEGER,
  limit_param INTEGER,
  offset_param INTEGER
) RETURNS TABLE(
  sender_id INTEGER,
  receiver_id INTEGER,
  initiator_id INTEGER,
  currency VARCHAR(64),
  amount BIGINT,
  fee BIGINT,
  created_at TIMESTAMP
) AS $$
BEGIN
  IF user_id_param != initiator_id_param
     AND NOT EXISTS (
       SELECT 1 FROM user_permission
       WHERE user_id = initiator_id_param
         AND permission_id IN (1, 6)
     ) THEN
    PERFORM raise_error(301);
END IF;

RETURN QUERY
SELECT
    transaction_logs.sender_id,
    transaction_logs.receiver_id,
    transaction_logs.initiator_id,
    transaction_logs.currency,
    transaction_logs.amount,
    transaction_logs.fee,
    transaction_logs.created_at
FROM transaction_logs
WHERE (transaction_logs.sender_id = user_id_param
    OR transaction_logs.receiver_id = user_id_param)
  AND transaction_logs.transaction_status = 100

UNION ALL

SELECT
    -1 AS sender_id,
    print_money_logs.receiver_id,
    print_money_logs.initiator_id,
    print_money_logs.currency,
    print_money_logs.amount,
    0 AS fee,
    print_money_logs.created_at
FROM print_money_logs
WHERE print_money_logs.receiver_id = user_id_param
  AND print_money_logs.print_status = 200

ORDER BY created_at DESC
OFFSET offset_param LIMIT limit_param;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION set_permission(
  initiator_id_param INTEGER,
  user_id_param INTEGER,
  permission_id_param INTEGER
) RETURNS VOID AS $$
BEGIN
  IF NOT EXISTS (
      SELECT 1 FROM permissions WHERE id = permission_id_param
  ) THEN
    PERFORM raise_error(401);
END IF;
  IF permission_id_param = 1 THEN
    PERFORM raise_error(401);
END IF;

  IF permission_id_param IN (2, 5)
     AND NOT EXISTS (
       SELECT 1 FROM user_permission
       WHERE user_id = initiator_id_param
         AND permission_id = 1
     ) THEN
    PERFORM raise_error(401);
END IF;

  IF NOT EXISTS (
      SELECT 1 FROM user_permission
      WHERE user_id = initiator_id_param
        AND permission_id IN (1, 2)
  ) THEN
    PERFORM raise_error(401);
END IF;

INSERT INTO user_permission (user_id, permission_id)
VALUES (user_id_param, permission_id_param)
    ON CONFLICT DO NOTHING;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION unset_permission(
  initiator_id_param INTEGER,
  user_id_param INTEGER,
  permission_id_param INTEGER
) RETURNS VOID AS $$
BEGIN
  IF NOT EXISTS (
      SELECT 1 FROM permissions WHERE id = permission_id_param
  ) THEN
    PERFORM raise_error(401);
END IF;

  IF permission_id_param = 1 THEN
    PERFORM raise_error(401);
END IF;

  IF permission_id_param IN (2, 5)
     AND NOT EXISTS (
       SELECT 1 FROM user_permission
       WHERE user_id = initiator_id_param
         AND permission_id = 1
     ) THEN
    PERFORM raise_error(401);
END IF;

  IF NOT EXISTS (
      SELECT 1 FROM user_permission
      WHERE user_id = initiator_id_param
        AND permission_id IN (1, 2)
  ) THEN
    PERFORM raise_error(401);
END IF;

DELETE FROM user_permission
WHERE user_id = user_id_param
  AND permission_id = permission_id_param;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION reset_user_password(
  initiator_id_param INTEGER,
  target_user_id_param INTEGER,
  new_password_hash_param CHAR(60)
) RETURNS VOID AS $$
BEGIN
  IF NOT EXISTS (
      SELECT 1 FROM users WHERE id = target_user_id_param
  ) THEN
    PERFORM raise_error(702);
END IF;

  IF NOT EXISTS (
      SELECT 1 FROM user_permission
      WHERE user_id = initiator_id_param
        AND permission_id IN (1, 4)
  ) THEN
    PERFORM raise_error(701);
END IF;

  IF EXISTS (
      SELECT 1 FROM user_permission
      WHERE user_id = target_user_id_param
        AND permission_id = 1
  ) THEN
    IF NOT EXISTS (
        SELECT 1 FROM user_permission
        WHERE user_id = initiator_id_param
          AND permission_id = 1
    ) THEN
      PERFORM raise_error(701);
END IF;
END IF;

UPDATE users
SET password_hash = new_password_hash_param
WHERE id = target_user_id_param;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION create_refresh_token(
  user_id_param INTEGER,
  expires_at_param TIMESTAMPTZ
) RETURNS UUID AS $$
DECLARE
new_token UUID;
BEGIN
INSERT INTO refresh_tokens(user_id, expires_at)
VALUES (user_id_param, expires_at_param)
    RETURNING token INTO new_token;

RETURN new_token;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION invalidate_refresh_tokens(
  user_id_param INTEGER
) RETURNS VOID AS $$
BEGIN
UPDATE refresh_tokens
SET revoked = true
WHERE user_id = user_id_param
  AND revoked = false
  AND expires_at > now();
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION is_refresh_token_valid(
  token_param UUID
) RETURNS INTEGER AS $$
DECLARE
valid_user INTEGER;
BEGIN
SELECT user_id
INTO valid_user
FROM refresh_tokens
WHERE token = token_param
  AND revoked = false
  AND expires_at > now()
    LIMIT 1;

IF valid_user IS NULL THEN
    RETURN -1;
ELSE
    RETURN valid_user;
END IF;
END;
$$ LANGUAGE plpgsql;
