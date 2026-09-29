-- Demo data only. Password for every account: DemoPass123!
-- Run after migrations. Existing demo emails are skipped without modification.
BEGIN;
DO $seed$
DECLARE
    names text[] := ARRAY['Анна', 'Мария', 'София', 'Алиса', 'Полина',
        'Екатерина', 'Дарья', 'Елизавета', 'Виктория', 'Анастасия',
        'Ксения', 'Варвара', 'Вероника', 'Александра', 'Ульяна'];
    tag_names text[] := ARRAY['музыка', 'спорт', 'путешествия', 'книги', 'кино', 'игры', 'кулинария', 'искусство'];
    demo_test_id bigint;
    tipi_test_id bigint;
    tipi_questions text[] := ARRAY[
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
    new_user_id bigint;
    new_profile_id bigint;
    new_tag_id bigint;
    email_value text;
    birth_date_value date;
    tag_name text;
    i integer;
    j integer;
BEGIN
    -- Serializes concurrent runs of this seed script.
    PERFORM pg_advisory_xact_lock(2026, 215);

    -- Демонстрационный русский перевод TIPI; порядок соответствует operation_id 1..10.
    -- Источник: https://gosling.psy.utexas.edu/scales-weve-developed/ten-item-personality-measure-tipi/ten-item-personality-inventory-tipi/
    -- Создаётся независимо от наличия demo-аккаунтов. Старые вопросы не переписываются.
    SELECT id INTO tipi_test_id FROM test
    WHERE name = 'TIPI — Краткий опросник Большой пятёрки (демо)' ORDER BY id LIMIT 1;
    IF tipi_test_id IS NULL THEN
        INSERT INTO test (name) VALUES ('TIPI — Краткий опросник Большой пятёрки (демо)')
        RETURNING id INTO tipi_test_id;
    END IF;
    FOR i IN 1..10 LOOP
        IF NOT EXISTS (SELECT 1 FROM question WHERE test_id = tipi_test_id AND operation_id = i) THEN
            INSERT INTO question (test_id, operation_id, body)
            VALUES (tipi_test_id, i, tipi_questions[i]);
        END IF;
    END LOOP;
    RAISE NOTICE 'TIPI готов: CURRENT_TEST_ID=%', tipi_test_id;

    FOR i IN 1..15 LOOP
        email_value := 'demo' || lpad(i::text, 2, '0') || '@example.com';
        IF EXISTS (SELECT 1 FROM "user" WHERE email = email_value) THEN
            RAISE NOTICE 'Skipping existing account %', email_value;
            CONTINUE;
        END IF;

        -- This fixture supports profile_psycho foreign keys, not GET/POST test flows.
        IF demo_test_id IS NULL THEN
            SELECT id INTO demo_test_id FROM test
            WHERE name = '[демо] Синтетические показатели Большой пятёрки' ORDER BY id LIMIT 1;
            IF demo_test_id IS NULL THEN
                INSERT INTO test (name) VALUES ('[демо] Синтетические показатели Большой пятёрки')
                RETURNING id INTO demo_test_id;
            END IF;
        END IF;

        birth_date_value := (CURRENT_DATE - make_interval(years => 21 + i % 10, months => i % 6))::date;
        INSERT INTO "user" (name, email, password_hash, birth_date)
        VALUES (names[i], email_value, '$2a$10$hYP67hh7aQCXCfqXgLZrFu2R5slhPE1OrXkoC4maAHgPDPg7SL/7G', birth_date_value)
        RETURNING id INTO new_user_id;

        INSERT INTO profile (user_id) VALUES (new_user_id)
        RETURNING id INTO new_profile_id;

        INSERT INTO profile_version
            (profile_id, revision, birth_date, sex, dating_goal, about_me,
            search_sex, search_age_from, search_age_to)
        VALUES (
            new_profile_id,
            1,
            birth_date_value,
            'female',
            CASE i % 3
                WHEN 0 THEN 'friendship'
                WHEN 1 THEN 'relationship'
                WHEN 2 THEN 'casual'
            END,
            'Тестовая анкета ' || names[i] || '. Люблю прогулки, музыку и новые знакомства.',
            'all',
            18,
            100
        );

        -- Synthetic normalized scores; each vector has differing components.
        INSERT INTO profile_psycho
            (profile_id, test_id, revision, openness, conscientiousness,
             extraversion, agreeableness, neuroticism)
        VALUES (new_profile_id, demo_test_id, 1,
            (20 + i * 7 % 71) / 100.0, (15 + i * 11 % 76) / 100.0,
            (10 + i * 13 % 81) / 100.0, (25 + i * 17 % 66) / 100.0,
            (5 + i * 19 % 86) / 100.0);

        INSERT INTO photo (profile_id, storage_key, position)
        VALUES (new_profile_id, 'demo/profile_' || lpad(i::text, 2, '0') || '.jpg', 1);

        -- Five tags for even profiles, six for odd profiles.
        FOR j IN 0..(4 + i % 2) LOOP
            tag_name := tag_names[1 + (i - 1 + j) % array_length(tag_names, 1)];
            INSERT INTO tag (name) VALUES (tag_name) ON CONFLICT (name) DO NOTHING;
            SELECT id INTO new_tag_id FROM tag WHERE name = tag_name;
            INSERT INTO profile_tag (profile_id, tag_id) VALUES (new_profile_id, new_tag_id);
        END LOOP;
    END LOOP;
END;
$seed$;

-- Counts only seed accounts; expected on first run: 15 rows, counts 1/1/1 and 5 or 6.
SELECT u.email, p.id AS profile_id,
    (SELECT count(*) FROM profile_version v WHERE v.profile_id = p.id) AS versions,
    (SELECT count(*) FROM profile_psycho ps WHERE ps.profile_id = p.id) AS psycho,
    (SELECT count(*) FROM photo ph WHERE ph.profile_id = p.id) AS photos,
    (SELECT count(*) FROM profile_tag pt WHERE pt.profile_id = p.id) AS tags
FROM "user" u JOIN profile p ON p.user_id = u.id
WHERE u.email IN (SELECT 'demo' || lpad(n::text, 2, '0') || '@example.com' FROM generate_series(1,15) n)
ORDER BY u.email;
SELECT id AS current_test_id, name FROM test
WHERE name = 'TIPI — Краткий опросник Большой пятёрки (демо)';
COMMIT;
