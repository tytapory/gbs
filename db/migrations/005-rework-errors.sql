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