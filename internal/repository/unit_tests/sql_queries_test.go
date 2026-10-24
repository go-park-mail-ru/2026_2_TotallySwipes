package repository

// Expected SQL is deliberately fixed in tests, independent of production constants.
const profileSelectSQL = `SELECT 
	p.id, p.user_id, p.created_at, p.updated_at,
	       EXISTS (SELECT 1 FROM search_filter WHERE user_id = p.user_id), l.city,
	       v.id, v.recorded_at, v.name, v.birth_date, v.sex, v.dating_goal, v.about_me,
	       v.education, v.work, v.smoking, v.alcohol, v.height,
	       ps.id, ps.recorded_at,
	       ps.openness, ps.conscientiousness, ps.extraversion, ps.agreeableness, ps.neuroticism
	FROM profile AS p
	LEFT JOIN LATERAL (
	    SELECT * FROM profile_version WHERE profile_id = p.id ORDER BY revision DESC LIMIT 1
	) AS v ON true
	LEFT JOIN LATERAL (
	    SELECT * FROM profile_psycho WHERE profile_id = p.id ORDER BY revision DESC LIMIT 1
	) AS ps ON true
    LEFT JOIN user_location AS l ON l.user_id = p.user_id`

const feedSQL = `SELECT 
	p.id, p.user_id, p.created_at, p.updated_at,
	       EXISTS (SELECT 1 FROM search_filter WHERE user_id = p.user_id), l.city,
	       v.id, v.recorded_at, v.name, v.birth_date, v.sex, v.dating_goal, v.about_me,
	       v.education, v.work, v.smoking, v.alcohol, v.height,
	       ps.id, ps.recorded_at,
	       ps.openness, ps.conscientiousness, ps.extraversion, ps.agreeableness, ps.neuroticism, 
	EXISTS (
		SELECT 1
		FROM swipe s
		JOIN profile viewer ON viewer.id = s.target_profile_id
		WHERE s.author_id = p.user_id
		AND viewer.user_id = $2
		AND s.action IN ('like', 'super_like')
	) AS liked_me
	FROM profile AS p
	LEFT JOIN LATERAL (
	    SELECT * FROM profile_version WHERE profile_id = p.id ORDER BY revision DESC LIMIT 1
	) AS v ON true
	LEFT JOIN LATERAL (
	    SELECT * FROM profile_psycho WHERE profile_id = p.id ORDER BY revision DESC LIMIT 1
	) AS ps ON true
    JOIN user_location AS l ON l.user_id = p.user_id
    JOIN user_location AS v_location ON v_location.user_id = $2 AND v_location.city = l.city
		WHERE p.user_id > $1 AND p.user_id <> $2
		  AND v.name IS NOT NULL AND v.birth_date IS NOT NULL AND v.sex IS NOT NULL
		  AND v.dating_goal IS NOT NULL
		  AND EXISTS (SELECT 1 FROM photo WHERE profile_id = p.id)
		  AND EXISTS (SELECT 1 FROM search_filter WHERE user_id = p.user_id)
		  AND NOT EXISTS (
		      SELECT 1 FROM search_filter AS f WHERE f.user_id = $2
		        AND ((f.sex <> 'all' AND f.sex <> v.sex)
		          OR date_part('year', age($4::date, v.birth_date)) NOT BETWEEN f.age_from AND f.age_to))
		ORDER BY p.user_id ASC LIMIT $3`

const lockProfileSQL = `SELECT id FROM profile WHERE user_id = $1 FOR UPDATE`

const photosSQL = `SELECT id, storage_key, position FROM photo WHERE profile_id = $1 ORDER BY position`

const tagsSQL = `SELECT t.id, t.name FROM tag AS t
		JOIN profile_tag AS pt ON pt.tag_id = t.id WHERE pt.profile_version_id = $1 ORDER BY t.id`

const insertVersionSQL = `INSERT INTO profile_version (
	    profile_id, revision, name, birth_date, sex, dating_goal, about_me,
	    education, work, smoking, alcohol, height)
	    SELECT $1, COALESCE(MAX(revision), 0) + 1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
	    FROM profile_version WHERE profile_id = $1 RETURNING id`

const touchSQL = `UPDATE profile SET updated_at = CURRENT_TIMESTAMP WHERE id = $1`

const reorderSQL = `UPDATE photo SET position = o.position, updated_at = CURRENT_TIMESTAMP
		 FROM unnest($2::bigint[]) WITH ORDINALITY AS o(id, position)
		 WHERE photo.id = o.id AND photo.profile_id = $1 AND photo.position <> o.position`

const shiftSQL = `UPDATE photo SET position = position - 1, updated_at = CURRENT_TIMESTAMP
		 WHERE profile_id = $1 AND position > $2`

const insertPsychoSQL = `
		INSERT INTO profile_psycho (
			profile_id, revision, openness, conscientiousness,
			extraversion, agreeableness, neuroticism, personality_type
		)
		SELECT $1, COALESCE(MAX(revision), 0) + 1, $2, $3, $4, $5, $6, $7
		FROM profile_psycho WHERE profile_id = $1
		RETURNING id, revision, recorded_at`

const getResultSQL = `
		SELECT id, revision, recorded_at, personality_type,
		       openness, conscientiousness, extraversion, agreeableness, neuroticism
		FROM profile_psycho
		WHERE profile_id = $1 AND openness IS NOT NULL
		ORDER BY revision DESC
		LIMIT 1`

const insertSwipeSQL = `
		INSERT INTO swipe (target_profile_id, author_id, action)
		VALUES ($1, $2, $3)
		ON CONFLICT (author_id, target_profile_id)
		DO UPDATE SET action = EXCLUDED.action, updated_at = CURRENT_TIMESTAMP`

const insertMatchSQL = `
		INSERT INTO match (first_user_id, second_user_id, access_to_message)
		SELECT LEAST($1::bigint, $2::bigint),
			GREATEST($1::bigint, $2::bigint),
			TRUE
		WHERE $3::text IN ('like', 'super_like')
		AND EXISTS (
			SELECT 1
			FROM swipe s
			JOIN profile p ON p.id = s.target_profile_id
			WHERE s.author_id = $2
				AND p.user_id = $1
				AND s.action IN ('like', 'super_like')
		)
		ON CONFLICT (first_user_id, second_user_id) DO NOTHING`
