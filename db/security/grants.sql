\set ON_ERROR_STOP on

SELECT current_database() AS database_name \gset

BEGIN;

REVOKE ALL PRIVILEGES ON DATABASE :"database_name" FROM PUBLIC;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;

-- Auth, Profile, Matching, Chat, OnlineConf и мониторинг подключаются к текущей базе данных.
GRANT CONNECT ON DATABASE :"database_name"
    TO auth_app, profile_app, matching_app, chat_app, onlineconf_app, monitor;

-- Auth, Profile, Matching, Chat и OnlineConf обращаются к таблицам в схеме public.
GRANT USAGE ON SCHEMA public
    TO auth_app, profile_app, matching_app, chat_app, onlineconf_app;

-- Auth читает учётные данные для входа и создаёт аккаунты при регистрации.
GRANT SELECT (id, email, password_hash), INSERT (email, password_hash)
    ON TABLE public."user"
    TO auth_app;

-- Auth создаёт пустую анкету в той же транзакции, что и аккаунт.
GRANT INSERT (user_id)
    ON TABLE public.profile
    TO auth_app;

-- Auth использует генераторы ID аккаунтов и пустых анкет при регистрации.
GRANT USAGE ON SEQUENCE public.user_id_seq, public.profile_id_seq
    TO auth_app;

-- Profile читает анкету и блокирует её строку при изменении анкеты или фотографий.
GRANT SELECT, UPDATE
    ON TABLE public.profile
    TO profile_app;

-- Profile читает и сохраняет новые версии анкеты, результаты тестов, ответы и теги версии.
GRANT SELECT, INSERT
    ON TABLE public.profile_version, public.profile_psycho,
             public.user_answer, public.profile_tag
    TO profile_app;

-- Profile получает доступные теги из справочника без возможности менять их.
GRANT SELECT ON TABLE public.tag
    TO profile_app;

-- Profile добавляет, переставляет и удаляет фотографии анкеты.
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE public.photo
    TO profile_app;

-- Profile читает и сохраняет фильтры поиска и местоположение пользователя.
GRANT SELECT, INSERT, UPDATE
    ON TABLE public.search_filter, public.user_location
    TO profile_app;

-- Profile пока читает свайпы и мэтчи в существующих запросах ленты и анкеты собеседника.
GRANT SELECT ON TABLE public.swipe, public."match"
    TO profile_app;

-- Profile использует генераторы ID версий анкеты, результатов тестов и фотографий.
GRANT USAGE ON SEQUENCE public.profile_version_id_seq,
                        public.profile_psycho_id_seq,
                        public.photo_id_seq
    TO profile_app;

-- Matching читает анкеты, результаты тестов, фото, теги, фильтры и геолокацию для ленты и списка мэтчей.
GRANT SELECT
    ON TABLE public.profile, public.profile_version, public.profile_psycho,
             public.photo, public.tag, public.profile_tag,
             public.search_filter, public.user_location
    TO matching_app;

-- Matching читает ID аккаунтов для блокировки строк при обработке конкурентных свайпов.
GRANT SELECT (id) ON TABLE public."user"
    TO matching_app;

-- Matching нужен UPDATE хотя бы одного столбца для существующего SELECT FOR UPDATE; пароли и статус аккаунта менять нельзя.
GRANT UPDATE (updated_at) ON TABLE public."user"
    TO matching_app;

-- Matching читает и сохраняет свайпы, создаёт мэтчи и отмечает их просмотренными.
GRANT SELECT, INSERT, UPDATE
    ON TABLE public.swipe, public."match"
    TO matching_app;

-- Matching читает, создаёт и снимает блокировки между пользователями.
GRANT SELECT, INSERT, DELETE ON TABLE public.user_block
    TO matching_app;

-- Matching использует генератор ID мэтчей при взаимной симпатии.
GRANT USAGE ON SEQUENCE public.match_id_seq
    TO matching_app;

-- Matching получает ID чата для каждого мэтча в списке симпатий.
GRANT SELECT (id, match_id) ON TABLE public.chat
    TO matching_app;

-- Chat читает и создаёт чаты, хранит состояние участников и обновляет счётчики позиций сообщений.
GRANT SELECT, INSERT, UPDATE
    ON TABLE public.chat, public.chat_member, public.chat_sequence, public.chat_user_state
    TO chat_app;

-- Chat читает историю и сохраняет сообщения; редактирование и удаление пока не реализуются.
GRANT SELECT, INSERT ON TABLE public.message
    TO chat_app;

-- Chat проверяет существование мэтча, разрешение переписки и блокировки пары через SQL.
GRANT SELECT ON TABLE public."match", public.user_block
    TO chat_app;

-- Chat проверяет бан аккаунтов, не получая доступа к почте и хешам паролей.
GRANT SELECT (id, is_disabled) ON TABLE public."user"
    TO chat_app;

-- Chat использует генераторы ID чатов и сообщений.
GRANT USAGE ON SEQUENCE public.chat_id_seq, public.message_id_seq
    TO chat_app;

-- Мониторинг читает статистику PostgreSQL без прав изменения прикладных таблиц.
GRANT pg_monitor TO monitor;

-- OnlineConf обращается к собственным таблицам без права создавать объекты схемы.
GRANT USAGE ON SCHEMA onlineconf TO onlineconf_app;

-- OnlineConf ищет пользователя по email, не получая доступ к хешам паролей.
GRANT SELECT (id, email) ON TABLE public."user" TO onlineconf_app;

-- OnlineConf создаёт и изменяет параметры и политики; конфигурации удаляются через deleted_at.
GRANT SELECT, INSERT, UPDATE
    ON TABLE onlineconf.configs, onlineconf.rollout_policies TO onlineconf_app;

-- OnlineConf управляет персональными исключениями и доступом администраторов.
GRANT SELECT, INSERT, UPDATE, DELETE
    ON TABLE onlineconf.flag_overrides, onlineconf.admins TO onlineconf_app;

-- OnlineConf добавляет и читает аудит без возможности менять или удалять историю.
GRANT SELECT, INSERT ON TABLE onlineconf.history TO onlineconf_app;

-- OnlineConf читает ревизию настроек и блокирует строку состояния в транзакциях изменения.
GRANT SELECT, UPDATE (revision) ON TABLE onlineconf.state TO onlineconf_app;

-- OnlineConf использует генераторы ID конфигураций, политик и записей истории.
GRANT USAGE ON ALL SEQUENCES IN SCHEMA onlineconf TO onlineconf_app;

COMMIT;
