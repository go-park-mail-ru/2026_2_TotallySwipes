-- Демонстрационное наполнение после 000001_init.up.sql.
-- 15 аккаунтов demo01@example.com ... demo15@example.com.
-- Пароль каждого аккаунта: DemoPass123!; в БД записывается bcrypt-хеш.
-- Все данные создаются одной транзакцией. Миграция выполняется один раз.
BEGIN;

DO $seed$
DECLARE
    names text[] := ARRAY[
        'Анна', 'Мария', 'София', 'Алиса', 'Полина',
        'Екатерина', 'Дарья', 'Елизавета', 'Виктория', 'Анастасия',
        'Ксения', 'Варвара', 'Вероника', 'Александра', 'Ульяна'
    ];
    descriptions text[] := ARRAY[
        'Люблю утренние пробежки, книги и разговоры за чашкой кофе.',
        'В свободное время хожу в театр и открываю новые места в городе.',
        'Рисую акварелью, путешествую и собираю впечатления.',
        'Люблю настольные игры, домашнюю выпечку и уютные вечера.',
        'Увлекаюсь фотографией и прогулками на природе.',
        'Хожу в походы, катаюсь на велосипеде и мечтаю увидеть Алтай.',
        'Занимаюсь йогой и люблю спокойные прогулки у воды.',
        'Играю на фортепиано и часто бываю на концертах.',
        'Люблю плавание, кино и встречи с друзьями.',
        'Делаю керамику и ищу вдохновение в музеях.',
        'Танцую, читаю романы и пробую новые рецепты.',
        'Выращиваю цветы и люблю проводить выходные за городом.',
        'Люблю длинные прогулки, выставки и хорошие истории.',
        'Путешествую по России и фотографирую архитектуру.',
        'Ценю чувство юмора, искренность и совместные приключения.'
    ];
    tag_names text[] := ARRAY[
        'Бег', 'Йога', 'Плавание', 'Велосипед', 'Походы',
        'Книги', 'Кино', 'Театр', 'Музеи', 'Классическая музыка',
        'Фотография', 'Рисование', 'Керамика', 'Танцы', 'Игра на музыкальных инструментах',
        'Путешествия', 'Кулинария', 'Кофейни', 'Настольные игры', 'Садоводство'
    ];
    -- Демонстрационный русский перевод TIPI из прежнего seed проекта,
    -- не валидированная адаптация TIPI-RU. Порядок соответствует operation_id 1..10.
    -- Оригинал: https://gosling.psy.utexas.edu/scales-weve-developed/ten-item-personality-measure-tipi/ten-item-personality-inventory-tipi/
    question_bodies text[] := ARRAY[
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
    tag_ids bigint[] := ARRAY[]::bigint[];
    tipi_id bigint;
    user_id_value bigint;
    profile_id_value bigint;
    inserted_id bigint;
    i integer;
    j integer;
BEGIN
    INSERT INTO test (name)
    VALUES ('TIPI — Краткий опросник Большой пятёрки (демо)')
    RETURNING id INTO tipi_id;

    FOR j IN 1..10 LOOP
        INSERT INTO question (test_id, operation_id, body)
        VALUES (tipi_id, j, question_bodies[j]);
    END LOOP;

    FOR j IN 1..20 LOOP
        INSERT INTO tag (name) VALUES (tag_names[j])
        RETURNING id INTO inserted_id;
        tag_ids := array_append(tag_ids, inserted_id);
    END LOOP;

    FOR i IN 1..15 LOOP
        INSERT INTO "user" (name, email, password_hash)
        VALUES (names[i], 'demo' || lpad(i::text, 2, '0') || '@example.com',
            '$2a$10$hYP67hh7aQCXCfqXgLZrFu2R5slhPE1OrXkoC4maAHgPDPg7SL/7G')
        RETURNING id INTO user_id_value;

        INSERT INTO profile (user_id) VALUES (user_id_value)
        RETURNING id INTO profile_id_value;

        INSERT INTO profile_version
            (profile_id, revision, birth_date, sex, dating_goal, about_me,
             search_sex, search_age_from, search_age_to)
        VALUES (profile_id_value, 1,
            (CURRENT_DATE - make_interval(years => 21 + i % 10, months => i % 6))::date,
            'female',
            CASE i % 3 WHEN 0 THEN 'friendship' WHEN 1 THEN 'relationship' ELSE 'casual' END,
            descriptions[i], 'all', 18, 45);

        -- Синтетические показатели от 0 до 1 для демонстрации ленты.
        -- Ответы на тест не создаются.
        INSERT INTO profile_psycho
            (profile_id, test_id, revision, openness, conscientiousness,
             extraversion, agreeableness, neuroticism)
        VALUES (profile_id_value, tipi_id, 1,
            (20 + i * 7 % 71) / 100.0,
            (15 + i * 11 % 76) / 100.0,
            (10 + i * 13 % 81) / 100.0,
            (25 + i * 17 % 66) / 100.0,
            (5 + i * 19 % 86) / 100.0);

        -- 5 тегов у чётных анкет, 6 у нечётных; без повторов внутри анкеты.
        FOR j IN 0..(4 + i % 2) LOOP
            INSERT INTO profile_tag (profile_id, tag_id)
            VALUES (profile_id_value, tag_ids[1 + ((i - 1) * 3 + j * 3) % 20]);
        END LOOP;
    END LOOP;

    RAISE NOTICE 'Демонстрационные данные созданы. Для API укажите CURRENT_TEST_ID=%', tipi_id;
END;
$seed$;

COMMIT;
