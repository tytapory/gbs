-- Таблица balances
CREATE INDEX IF NOT EXISTS balances_user_id_idx
    ON balances(user_id);

CREATE INDEX IF NOT EXISTS balances_user_currency_idx
    ON balances(user_id, currency);

CREATE INDEX IF NOT EXISTS balances_currency_idx
    ON balances(currency);

-- Таблица permissions
CREATE INDEX IF NOT EXISTS permissions_name_idx
    ON permissions(name);

-- Таблица user_permission
CREATE INDEX IF NOT EXISTS user_permission_user_id_idx
    ON user_permission(user_id);

CREATE INDEX IF NOT EXISTS user_permission_permission_id_idx
    ON user_permission(permission_id);

CREATE INDEX IF NOT EXISTS user_permission_user_id_permission_id_idx
    ON user_permission(user_id, permission_id);

-- Таблица recovery_code
CREATE INDEX IF NOT EXISTS recovery_code_user_id_idx
    ON recovery_code(user_id);

-- Таблица transaction_logs
CREATE INDEX IF NOT EXISTS transaction_logs_sender_id_idx
    ON transaction_logs(sender_id);

CREATE INDEX IF NOT EXISTS transaction_logs_receiver_id_idx
    ON transaction_logs(receiver_id);

CREATE INDEX IF NOT EXISTS transaction_logs_initiator_id_idx
    ON transaction_logs(initiator_id);

CREATE INDEX IF NOT EXISTS transaction_logs_currency_idx
    ON transaction_logs(currency);

CREATE INDEX IF NOT EXISTS transaction_logs_created_at_idx
    ON transaction_logs(created_at);

CREATE INDEX IF NOT EXISTS transaction_logs_sender_receiver_idx
    ON transaction_logs(sender_id, receiver_id);

CREATE INDEX IF NOT EXISTS transaction_logs_status_idx
    ON transaction_logs(transaction_status);

-- Таблица print_money_logs
CREATE INDEX IF NOT EXISTS print_money_logs_initiator_id_idx
    ON print_money_logs(initiator_id);

CREATE INDEX IF NOT EXISTS print_money_logs_receiver_id_idx
    ON print_money_logs(receiver_id);

CREATE INDEX IF NOT EXISTS print_money_logs_currency_idx
    ON print_money_logs(currency);

CREATE INDEX IF NOT EXISTS print_money_logs_created_at_idx
    ON print_money_logs(created_at);

CREATE INDEX IF NOT EXISTS print_money_logs_initiator_receiver_idx
    ON print_money_logs(initiator_id, receiver_id);

CREATE INDEX IF NOT EXISTS print_money_logs_print_status_idx
    ON print_money_logs(print_status);

CREATE UNIQUE INDEX IF NOT EXISTS idx_refresh_tokens_token
    ON refresh_tokens(token);