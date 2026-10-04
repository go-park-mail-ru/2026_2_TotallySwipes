-- На новой базе поле и ограничения уже создаются в 000001.
-- Миграция сохранена для баз, созданных прежней версией 000001.
BEGIN;
ALTER TABLE "profile_psycho" ADD COLUMN IF NOT EXISTS "personality_type" TEXT NULL;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'profile_psycho'::regclass AND conname = 'profile_psycho_personality_type_valid'
    ) THEN
        ALTER TABLE profile_psycho ADD CONSTRAINT "profile_psycho_personality_type_valid" CHECK (personality_type IN (
        'EXPLORER', 'VISIONARY', 'STRATEGIST', 'INVENTOR', 'DREAMER', 'CURATOR', 'INSPIRER', 'DEBATER',
        'ORGANIZER', 'CONNECTOR', 'COMPANION', 'DRIVER', 'ANCHOR', 'CRAFTSPERSON', 'OBSERVER', 'KEEPER'
    ));
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'profile_psycho'::regclass AND conname = 'profile_psycho_personality_type_needs_scores'
    ) THEN
        ALTER TABLE profile_psycho ADD CONSTRAINT "profile_psycho_personality_type_needs_scores" CHECK (personality_type IS NULL OR openness IS NOT NULL);
    END IF;
END;
$$;
COMMIT;
