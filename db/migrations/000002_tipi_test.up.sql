DO $$
DECLARE
    tipi_name text := 'TIPI — Краткий опросник Большой пятёрки';
    existing_name text;
    questions text[] := ARRAY[
        'Я считаю себя общительным человеком, полным энтузиазма.',
        'Я считаю себя критичным человеком, склонным к спорам.',
        'Я считаю себя надёжным и дисциплинированным человеком.',
        'Я считаю себя тревожным человеком, которого легко расстроить.',
        'Я считаю себя открытым новому опыту человеком с разносторонними интересами.',
        'Я считаю себя сдержанным и немногословным человеком.',
        'Я считаю себя отзывчивым и сердечным человеком.',
        'Я считаю себя неорганизованным и небрежным человеком.',
        'Я считаю себя спокойным и эмоционально устойчивым человеком.',
        'Я считаю себя приверженцем привычного, с небольшим интересом к творчеству.'
    ];
BEGIN
    SELECT name INTO existing_name FROM test WHERE id = 1;

    IF existing_name IS NULL THEN
        INSERT INTO test (id, name) OVERRIDING SYSTEM VALUE VALUES (1, tipi_name);
    ELSIF existing_name LIKE 'TIPI%' THEN
        UPDATE test SET name = tipi_name, updated_at = CURRENT_TIMESTAMP WHERE id = 1 AND name <> tipi_name;
    ELSE
        RAISE EXCEPTION 'test id=1 is %, expected TIPI', existing_name;
    END IF;

    FOR i IN 1..10 LOOP
        IF NOT EXISTS (SELECT 1 FROM question WHERE test_id = 1 AND operation_id = i) THEN
            INSERT INTO question (test_id, operation_id, body) VALUES (1, i, questions[i]);
        END IF;
    END LOOP;

    PERFORM setval(pg_get_serial_sequence('test', 'id'), (SELECT max(id) FROM test));
END
$$;
