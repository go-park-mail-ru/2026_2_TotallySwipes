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
    
    question_bodies text[] := ARRAY[
        'общительным человеком, полным энтузиазма.',
        'критичным человеком, склонным к спорам.',
        'надёжным и дисциплинированным человеком.',
        'тревожным человеком, которого легко расстроить.',
        'открытым новому опыту человеком с разносторонними интересами.',
        'сдержанным и немногословным человеком.',
        'отзывчивым и сердечным человеком.',
        'неорганизованным и небрежным человеком.',
        'спокойным и эмоционально устойчивым человеком.',
        'приверженцем привычного, с небольшим интересом к творчеству.'
    ];
    tag_ids bigint[] := ARRAY[]::bigint[];
    question_ids bigint[] := ARRAY[]::bigint[];
    answer_values integer[] := ARRAY[]::integer[];
    tipi_id bigint;
    user_id_value bigint;
    profile_id_value bigint;
    profile_version_id_value bigint;
    psycho_id_value bigint;
    inserted_id bigint;
    i integer;
    j integer;
BEGIN
    INSERT INTO test (name)
    VALUES ('TIPI — Краткий опросник Большой пятёрки (демо)')
    RETURNING id INTO tipi_id;

    FOR j IN 1..10 LOOP
        INSERT INTO question (test_id, operation_id, body)
        VALUES (tipi_id, j, question_bodies[j])
        RETURNING id INTO inserted_id;
        question_ids := array_append(question_ids, inserted_id);
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
            descriptions[i], 'all', 18, 45)
        RETURNING id INTO profile_version_id_value;

        INSERT INTO profile_psycho (profile_id, test_id, revision)
        VALUES (profile_id_value, tipi_id, 1)
        RETURNING id INTO psycho_id_value;

        answer_values := ARRAY[]::integer[];
        FOR j IN 1..10 LOOP
            answer_values := array_append(answer_values, 1 + (i * 3 + (j - 1) * 5) % 7);
            INSERT INTO user_answer (profile_psycho_id, question_id, answer_value)
            VALUES (psycho_id_value, question_ids[j], answer_values[j]);
        END LOOP;

        -- TIPI: ((direct + 8 - reverse) / 2 - 1) / 6.
        -- Деление на 12.0 даёт нормализованные значения [0, 1] без целочисленного усечения.
        -- Индексы массива соответствуют operation_id, а не сгенерированным question.id.
        -- Все показатели заполняются одним UPDATE; последующие UPDATE запретит триггер.
        UPDATE profile_psycho
           SET openness = (answer_values[5] + 6 - answer_values[10]) / 12.0,
               conscientiousness = (answer_values[3] + 6 - answer_values[8]) / 12.0,
               extraversion = (answer_values[1] + 6 - answer_values[6]) / 12.0,
               agreeableness = (answer_values[7] + 6 - answer_values[2]) / 12.0,
               neuroticism = (answer_values[4] + 6 - answer_values[9]) / 12.0
         WHERE id = psycho_id_value;

        -- 5 тегов у чётных анкет, 6 у нечётных; без повторов внутри анкеты.
        FOR j IN 0..(4 + i % 2) LOOP
            INSERT INTO profile_tag (profile_version_id, tag_id)
            VALUES (profile_version_id_value, tag_ids[1 + ((i - 1) * 3 + j * 3) % 20]);
        END LOOP;
    END LOOP;

    RAISE NOTICE 'Демонстрационные данные созданы. Для API укажите CURRENT_TEST_ID=%', tipi_id;
END;
$seed$;

COMMIT;
