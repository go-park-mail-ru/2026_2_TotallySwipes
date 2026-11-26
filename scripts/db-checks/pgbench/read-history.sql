\set chat_id random(900000001, 900000010)

BEGIN;

SELECT id, seq, sender_id, type, body, created_at, version
FROM (
    SELECT id, seq, sender_id, type, body, created_at, version
    FROM public.message
    WHERE chat_id = :chat_id
    ORDER BY seq DESC
    LIMIT 50
) AS recent_messages
ORDER BY seq ASC;

COMMIT;
