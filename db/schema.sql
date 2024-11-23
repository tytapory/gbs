CREATE TABLE users
(
	id SERIAL PRIMARY KEY,
	username VARCHAR(64) NOT NULL UNIQUE,
	password_hash VARCHAR(64),
	created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE balances
(
	user_id INTEGER NOT NULL REFERENCES users(id),
	currency VARCHAR(64) NOT NULL,
	amount BIGINT NOT NULL
);

CREATE TABLE permissions
(
	id SERIAL PRIMARY KEY,
	name VARCHAR(32)
);

CREATE TABLE user_permission
(
	user_id INTEGER NOT NULL REFERENCES users(id),
	permission_id INTEGER NOT NULL REFERENCES permissions(id)
);

CREATE TABLE recovery_code
(
	user_id INTEGER NOT NULL REFERENCES users(id),
	code VARCHAR(12) NOT NULL,
	valid_until TIMESTAMP NOT NULL
);

CREATE TABLE transfer_logs
(
	id SERIAL PRIMARY KEY,
	sender_id INTEGER NOT NULL REFERENCES users(id),
	receiver_id INTEGER NOT NULL REFERENCES users(id),
	sender_balance_after BIGINT NOT NULL,
	receiver_balance_after BIGINT NOT NULL,
	currency VARCHAR(64) NOT NULL,
	amount BIGINT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
