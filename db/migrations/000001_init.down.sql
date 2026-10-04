BEGIN;

DROP TABLE "message";
DROP TABLE "match";
DROP TABLE "profile_like";
DROP TABLE "photo_comment";
DROP TABLE "photo";
DROP TABLE "report";
DROP TABLE "report_type";
DROP TABLE "profile_tag";
DROP TABLE "tag";
DROP TABLE "subscription";
DROP TABLE "plan";
DROP TABLE "user_answer";
DROP TABLE "profile_psycho";
DROP TABLE "question";
DROP TABLE "test";
DROP TABLE "profile_version";
DROP TABLE "profile";
DROP TABLE "user";

DROP FUNCTION protect_history_delete();
DROP FUNCTION protect_completed_psycho();
DROP FUNCTION reject_history_update();
DROP FUNCTION lock_answer_question();
DROP FUNCTION protect_answered_question();

COMMIT;
