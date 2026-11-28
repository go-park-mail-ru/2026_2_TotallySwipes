BEGIN;

DROP TABLE onlineconf.flag_overrides;
DROP TABLE onlineconf.history;
DROP TABLE onlineconf.admins;
DROP TABLE onlineconf.configs;
DROP TABLE onlineconf.rollout_policies;
DROP TABLE onlineconf.state;
DROP SCHEMA onlineconf;

COMMIT;
