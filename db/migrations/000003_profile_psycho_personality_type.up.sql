ALTER TABLE "profile_psycho"
    ADD COLUMN "personality_type" TEXT NULL,
    ADD CONSTRAINT "profile_psycho_personality_type_valid" CHECK (personality_type IN (
        'EXPLORER', 'VISIONARY', 'STRATEGIST', 'INVENTOR', 'DREAMER', 'CURATOR', 'INSPIRER', 'DEBATER',
        'ORGANIZER', 'CONNECTOR', 'COMPANION', 'DRIVER', 'ANCHOR', 'CRAFTSPERSON', 'OBSERVER', 'KEEPER'
    )),
    ADD CONSTRAINT "profile_psycho_personality_type_needs_scores" CHECK (personality_type IS NULL OR openness IS NOT NULL);
