CREATE INDEX IF NOT EXISTS transaction_logs_sender_id_idx
    ON transaction_logs(sender_id);

CREATE INDEX IF NOT EXISTS transaction_logs_receiver_id_idx
    ON transaction_logs(receiver_id);

CREATE INDEX IF NOT EXISTS transaction_logs_initiator_id_idx
    ON transaction_logs(initiator_id);

CREATE INDEX IF NOT EXISTS transaction_logs_created_at_desc_idx
    ON transaction_logs(created_at DESC);

CREATE INDEX IF NOT EXISTS refresh_tokens_user_id_idx
    ON refresh_tokens(user_id);