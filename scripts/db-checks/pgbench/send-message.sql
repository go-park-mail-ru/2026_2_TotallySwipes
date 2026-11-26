\set chat_id random(900000001, 900000010)

BEGIN;

WITH next_position AS (
    UPDATE public.chat_sequence
    SET last_seq = last_seq + 1
    WHERE chat_id = :chat_id
    RETURNING chat_id, last_seq
)
INSERT INTO public.message (chat_id, seq, sender_id, client_message_id, type, body)
SELECT chat_id,
       last_seq,
       910000001,
       'pgbench-' || chat_id || '-' || last_seq,
       'text',
       'Сообщение от pgbench'
FROM next_position;

COMMIT;
