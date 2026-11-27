package postgres_test

const lockSwipeQuery = `
    SELECT pg_advisory_xact_lock(hashtextextended(
        'matching-pair:' || LEAST($1::bigint, $2::bigint)::text || ':' || GREATEST($1::bigint, $2::bigint)::text, 0
    ))`

const upsertSwipeQuery = `
    INSERT INTO swipe (target_profile_id, author_id, action)
    VALUES ($1, $2, $3)
    ON CONFLICT (author_id, target_profile_id)
    DO UPDATE SET action = EXCLUDED.action, updated_at = CURRENT_TIMESTAMP`

const insertMutualMatchQuery = `
    INSERT INTO match (first_user_id, second_user_id, access_to_message)
    SELECT LEAST($1::bigint, $2::bigint), GREATEST($1::bigint, $2::bigint), TRUE
    WHERE $3::text IN ('like', 'super_like')
      AND EXISTS (
          SELECT 1 FROM swipe
          WHERE author_id = $2 AND target_profile_id = $4 AND action IN ('like', 'super_like')
      )
    ON CONFLICT (first_user_id, second_user_id) DO NOTHING`

const selectMatchIDQuery = `
    SELECT id
    FROM match
    WHERE first_user_id = LEAST($1::bigint, $2::bigint)
      AND second_user_id = GREATEST($1::bigint, $2::bigint)
      AND $3::text IN ('like', 'super_like')`

const getParticipantsQuery = `
							SELECT first_user_id, second_user_id
							FROM match
							WHERE id = $1 AND (first_user_id = $2 OR second_user_id = $2);`

const countMatchesQuery = `
		SELECT count(*) FROM match WHERE first_user_id = $1 OR second_user_id = $1`

const selectMatchesQuery = `
    SELECT matching.id, chat.id, matching.created_at,
           CASE WHEN matching.first_user_id = $1 THEN matching.second_user_id ELSE matching.first_user_id END,
           CASE WHEN matching.first_user_id = $1 THEN matching.first_user_seen_at ELSE matching.second_user_seen_at END IS NULL
    FROM match AS matching
    LEFT JOIN chat ON chat.match_id = matching.id
    WHERE (matching.first_user_id = $1 OR matching.second_user_id = $1) AND matching.id < $2
    ORDER BY matching.id DESC LIMIT $3`

const markMatchSeenQuery = `
		UPDATE match SET
		    first_user_seen_at = CASE WHEN first_user_id = $1 THEN COALESCE(first_user_seen_at, CURRENT_TIMESTAMP) ELSE first_user_seen_at END,
		    second_user_seen_at = CASE WHEN second_user_id = $1 THEN COALESCE(second_user_seen_at, CURRENT_TIMESTAMP) ELSE second_user_seen_at END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $2 AND (first_user_id = $1 OR second_user_id = $1)`

const selectMatchSummaryQuery = `
		SELECT count(*),
		       count(*) FILTER (WHERE CASE WHEN first_user_id = $1 THEN first_user_seen_at ELSE second_user_seen_at END IS NULL)
		FROM match WHERE first_user_id = $1 OR second_user_id = $1`

const getMatchIDQuery = `
    SELECT id FROM match
    WHERE first_user_id = LEAST($1::bigint, $2::bigint)
      AND second_user_id = GREATEST($1::bigint, $2::bigint)`

const selectSwipeStatesQuery = `
    SELECT candidates.user_id,
           EXISTS (SELECT 1 FROM swipe WHERE author_id = $1 AND target_profile_id = candidates.profile_id),
           EXISTS (SELECT 1 FROM swipe WHERE author_id = candidates.user_id AND target_profile_id = $4 AND action IN ('like', 'super_like'))
    FROM unnest($2::bigint[], $3::bigint[]) AS candidates(user_id, profile_id)`
