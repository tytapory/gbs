CREATE TABLE users (
	id SERIAL PRIMARY KEY,
	username VARCHAR(64) NOT NULL UNIQUE,
	password_hash VARCHAR(64),
	created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE balances (
	user_id INTEGER NOT NULL REFERENCES users(id),
	currency VARCHAR(64) NOT NULL,
	amount BIGINT NOT NULL
);

CREATE TABLE permissions (
	id SERIAL PRIMARY KEY,
	name VARCHAR(32) NOT NULL UNIQUE
);

CREATE TABLE user_permission (
	user_id INTEGER NOT NULL REFERENCES users(id),
	permission_id INTEGER NOT NULL REFERENCES permissions(id)
);

CREATE TABLE recovery_code (
	user_id INTEGER NOT NULL REFERENCES users(id),
	code VARCHAR(12) NOT NULL,
	valid_until TIMESTAMP NOT NULL
);

CREATE TABLE transfer_logs (
	id SERIAL PRIMARY KEY,
	sender_id INTEGER NOT NULL REFERENCES users(id),
	receiver_id INTEGER NOT NULL REFERENCES users(id),
	sender_balance_after BIGINT NOT NULL,
	receiver_balance_after BIGINT NOT NULL,
	currency VARCHAR(64) NOT NULL,
	amount BIGINT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

INSERT INTO
	permissions(name)
VALUES
	('administrator'),
	('manage_user_permissions'),
	('manage_user_funds'),
	('control_user_accounts'),
	('print_money'),
	('audit_funds'),
	('receive_funds'),
	('send_funds');

CREATE
OR REPLACE FUNCTION proceed_transaction(
	sender_id INTEGER,
	receiver_id INTEGER,
	initiator_id INTEGER,
	currency VARCHAR(64),
	amount BIGINT
) RETURNS INTEGER AS $ $ DECLARE sender_balance BIGINT;

receiver_balance BIGINT;

BEGIN IF initiator_id IS NOT sender_id THEN IF NOT EXISTS (
	SELECT
		user_id
	FROM
		user_permission
		JOIN permissions ON permissions.id = user_permission.permission_id
	WHERE
		user_id = initiator_id
		AND (
			permissions.name = 'manage_user_funds'
			OR permissions.name = 'administrator'
		)
) THEN RETURN 1 --initiator id not equal to sender id insufficient rights
END IF;

ELSE IF NOT EXISTS (
	SELECT
		user_id
	FROM
		user_permission
		JOIN permissions ON permissions.id = user_permission.permission_id
	WHERE
		user_id = initiator_id
		AND (
			permissions.name = 'send_funds'
			OR permissions.name = 'manage_user_funds'
			OR permissions.name = 'administrator'
		)
) THEN RETURN 2 --sender id insufficient rights
END IF;

END IF;

IF NOT EXISTS (
	SELECT
		user_id
	FROM
		user_permission
		JOIN permissions ON permissions.id = user_permission.permission_id
	WHERE
		user_id = receiver_id
		AND (
			permissions.name = 'send_funds'
			OR permissions.name = 'manage_user_funds'
			OR permissions.name = 'administrator'
		)
) THEN RETURN 3 --receiver id insufficient rights
END IF;

SELECT
	amount INTO sender_balance
FROM
	balances
WHERE
	balances.user_id = sender_id
	AND currency = currency;

IF sender_balance < amount THEN RETURN 4;

--insufficient funds
END IF;

SELECT
	amount INTO receiver_balance
FROM
	balances
WHERE
	balances.user_id = receiver_id
	AND balances.currency = currency;

IF receiver_balance IS NULL THEN
INSERT INTO
	balances(user_id, currency, amount)
VALUES
	(receiver_id, currency, amount);

ELSE
UPDATE
	balances
SET
	amount = receiver_balance + amount
WHERE
	user_id = receiver_id
	AND currency = currency;

END IF;

UPDATE
	balances
SET
	amount = sender_balance - amount
WHERE
	user_id = sender_id
	AND currency = currency;

RETURN 0;

--all good
END;

$ $ LANGUAGE plpgsql;