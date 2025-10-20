ALTER TABLE error_description
    ADD COLUMN sql_state CHAR(5) NOT NULL DEFAULT 'P0001';

UPDATE error_description SET sql_state = 'G0001'
WHERE code IN (101, 102, 103, 201, 202, 302, 702);

UPDATE error_description SET sql_state = 'G0002'
WHERE code IN (104, 105, 106, 203, 301, 501, 601, 701);

UPDATE error_description SET sql_state = 'G0003'
WHERE code IN (107, 108, 204);

UPDATE error_description SET sql_state = 'G0004'
WHERE code = 401;

CREATE OR REPLACE FUNCTION raise_error(code_param integer) RETURNS void AS $$
DECLARE error_text text;
        state_code CHAR(5);
BEGIN
SELECT description, sql_state
INTO error_text, state_code
FROM error_description
WHERE error_description.code = code_param;

IF error_text IS NULL THEN
      error_text := 'Unknown error';
      state_code := 'P0001';
END IF;
  RAISE EXCEPTION '%', error_text USING ERRCODE = state_code;
END;
$$ LANGUAGE plpgsql;

CREATE INDEX IF NOT EXISTS error_description_code_idx
    ON error_description(code);

CREATE OR REPLACE FUNCTION register_user(username_param text, password_hash_param text)
RETURNS integer AS $$
DECLARE new_user_id integer;
BEGIN
IF EXISTS (
  SELECT 1 FROM users WHERE users.username = username_param
) THEN PERFORM raise_error(401);
END IF;
INSERT INTO users (username, password_hash)
VALUES (username_param, password_hash_param)
    RETURNING id INTO new_user_id;
RETURN new_user_id;
END;
$$ LANGUAGE plpgsql;

INSERT INTO error_description (code, description, sql_state)
VALUES (801, 'Invalid or expired refresh token', 'G0001');

CREATE OR REPLACE FUNCTION is_refresh_token_valid(token_param UUID)
RETURNS INTEGER AS $$
DECLARE valid_user INTEGER;
BEGIN
SELECT user_id
INTO valid_user
FROM refresh_tokens
WHERE token = token_param
  AND revoked = false
  AND expires_at > now()
    LIMIT 1;

IF valid_user IS NULL THEN
    PERFORM raise_error(801);
END IF;

RETURN valid_user;
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
    PERFORM raise_error(601);
END IF;
  IF permission_id_param = 1 THEN
    PERFORM raise_error(601);
END IF;

  IF permission_id_param IN (2, 5)
     AND NOT EXISTS (
       SELECT 1 FROM user_permission
       WHERE user_id = initiator_id_param
         AND permission_id = 1
     ) THEN
    PERFORM raise_error(601);
END IF;

  IF NOT EXISTS (
      SELECT 1 FROM user_permission
      WHERE user_id = initiator_id_param
        AND permission_id IN (1, 2)
  ) THEN
    PERFORM raise_error(601);
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
    PERFORM raise_error(601);
END IF;

  IF permission_id_param = 1 THEN
    PERFORM raise_error(601);
END IF;

  IF permission_id_param IN (2, 5)
     AND NOT EXISTS (
       SELECT 1 FROM user_permission
       WHERE user_id = initiator_id_param
         AND permission_id = 1
     ) THEN
    PERFORM raise_error(601);
END IF;

  IF NOT EXISTS (
      SELECT 1 FROM user_permission
      WHERE user_id = initiator_id_param
        AND permission_id IN (1, 2)
  ) THEN
    PERFORM raise_error(601);
END IF;

DELETE FROM user_permission
WHERE user_id = user_id_param
  AND permission_id = permission_id_param;
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
receiver_balance_after bigint;
BEGIN

  IF NOT EXISTS (SELECT 1 FROM users WHERE id = receiver_id_param) THEN
    PERFORM raise_error(201);
END IF;

  IF NOT EXISTS (SELECT 1 FROM users WHERE id = initiator_id_param) THEN
    PERFORM raise_error(202);
END IF;

  IF NOT EXISTS (
      SELECT 1 FROM user_permission
     WHERE user_id = initiator_id_param
       AND permission_id IN (1, 5)
  ) THEN
    PERFORM raise_error(203);
END IF;

  IF amount_param <= 0 THEN
    PERFORM raise_error(204);
END IF;

INSERT INTO balances(user_id, currency, amount)
VALUES (receiver_id_param, currency_param, amount_param)
    ON CONFLICT (user_id, currency)
      DO UPDATE SET amount = balances.amount + EXCLUDED.amount
                 RETURNING balances.amount INTO receiver_balance_after;

PERFORM log_print_money(
      receiver_id_param, initiator_id_param, 200, receiver_balance_after,
      currency_param, amount_param
  );
END;
$$ LANGUAGE plpgsql;

INSERT INTO error_description (code, description, sql_state)
VALUES (109, 'Transaction: Cannot send funds to self', 'G0003');

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
sender_balance_old bigint;
  sender_balance_new bigint;
  receiver_balance_new bigint;
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

  IF sender_id_param = receiver_id_param THEN
    PERFORM raise_error(109);
END IF;

SELECT amount
INTO sender_balance_old
FROM balances
WHERE user_id = sender_id_param AND currency = currency_param
    FOR UPDATE;

PERFORM check_transaction_permissions(initiator_id_param, sender_id_param, receiver_id_param);

  IF amount_param <= 0 THEN
    PERFORM raise_error(108);
END IF;

  IF sender_balance_old < amount_param OR sender_balance_old IS NULL THEN
    PERFORM raise_error(107);
END IF;

  commission_amount := (amount_param * fee_param + 9999) / 10000;

INSERT INTO balances(user_id, currency, amount)
VALUES (
           receiver_id_param, currency_param, amount_param - commission_amount
       )
    ON CONFLICT (user_id, currency)
      DO UPDATE SET amount = balances.amount + EXCLUDED.amount
                 RETURNING amount INTO receiver_balance_new;

INSERT INTO balances(user_id, currency, amount)
VALUES (
           2, currency_param, commission_amount
       )
    ON CONFLICT (user_id, currency)
      DO UPDATE SET amount = balances.amount + EXCLUDED.amount;

UPDATE balances
SET amount = sender_balance_old - amount_param
WHERE user_id = sender_id_param AND currency = currency_param
    RETURNING amount INTO sender_balance_new;

PERFORM log_transaction(
      sender_id_param, receiver_id_param, initiator_id_param, 100,
      sender_balance_new, receiver_balance_new,
      currency_param, amount_param, commission_amount
  );
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
    PERFORM raise_error(501);
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

WHERE (print_money_logs.receiver_id = user_id_param OR print_money_logs.initiator_id = user_id_param)
  AND print_money_logs.print_status = 200

ORDER BY created_at DESC
OFFSET offset_param LIMIT limit_param;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION get_amount_of_user_transactions(
  initiator_id_param integer,
  user_id_param integer

RETURNS integer AS $$
DECLARE
transaction_count INTEGER;
BEGIN
  IF NOT EXISTS (
      SELECT 1 FROM user_permission
      WHERE initiator_id_param = user_id AND
            (permission_id = 1 OR permission_id = 6)
  ) AND user_id_param != initiator_id_param THEN
    PERFORM raise_error(501);
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

INSERT INTO error_description (code, description, sql_state)
VALUES (402, 'Registration: Insufficient permissions', 'G0002');

INSERT INTO error_description (code, description, sql_state)
VALUES (403, 'Registration: Public registration is disabled', 'G0002');

CREATE OR REPLACE FUNCTION register_user(
  initiator_id_param INTEGER,
  username_param text,
  password_hash_param text,
  public_reg_allowed_param BOOLEAN
)
RETURNS integer AS $$
DECLARE
new_user_id integer;
BEGIN

  IF initiator_id_param = 0 THEN
    IF NOT public_reg_allowed_param THEN
      PERFORM raise_error(403);
    END IF;

  ELSE
    IF NOT EXISTS (
        SELECT 1 FROM user_permission
        WHERE user_id = initiator_id_param
          AND permission_id IN (1, 4)
    ) THEN
      PERFORM raise_error(402);
    END IF;

  END IF;

  IF EXISTS (
      SELECT 1 FROM users WHERE users.username = username_param
  ) THEN
    PERFORM raise_error(401);
  END IF;

  INSERT INTO users (username, password_hash)
  VALUES (username_param, password_hash_param)
      RETURNING id INTO new_user_id;

  RETURN new_user_id;
END;
$$ LANGUAGE plpgsql;
