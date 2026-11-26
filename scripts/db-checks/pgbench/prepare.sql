BEGIN;

DELETE FROM public.message
WHERE chat_id BETWEEN 900000001 AND 900000010;

INSERT INTO public.chat (id, match_id)
OVERRIDING SYSTEM VALUE
SELECT id, id
FROM generate_series(900000001::bigint, 900000010::bigint) AS id
ON CONFLICT (id) DO NOTHING;

INSERT INTO public.chat_member (chat_id, user_id)
SELECT chat.id, member.user_id
FROM public.chat
CROSS JOIN (VALUES (910000001::bigint), (910000002::bigint)) AS member(user_id)
WHERE chat.id BETWEEN 900000001 AND 900000010
ON CONFLICT (chat_id, user_id)
DO UPDATE SET seen_at = NULL, last_read_seq = 0;

INSERT INTO public.chat_sequence (chat_id, last_seq)
SELECT id, 1000
FROM public.chat
WHERE id BETWEEN 900000001 AND 900000010
ON CONFLICT (chat_id)
DO UPDATE SET last_seq = EXCLUDED.last_seq;

INSERT INTO public.message (chat_id, seq, sender_id, type, body)
SELECT chat.id,
       position.seq,
       CASE WHEN position.seq % 2 = 0 THEN 910000001 ELSE 910000002 END,
       'text',
       'Тестовое сообщение ' || position.seq
FROM public.chat
CROSS JOIN generate_series(1, 1000) AS position(seq)
WHERE chat.id BETWEEN 900000001 AND 900000010;

COMMIT;

ANALYZE public.chat;
ANALYZE public.chat_member;
ANALYZE public.chat_sequence;
ANALYZE public.message;
