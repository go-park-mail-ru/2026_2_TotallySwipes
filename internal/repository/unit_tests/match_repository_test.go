package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"dating-app/internal/model"
	"dating-app/internal/repository"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestMatchRepositoryGetMatches(t *testing.T) {
	dsn := os.Getenv("TEST_HISTORY_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_HISTORY_DATABASE_URL is not set")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	users := repository.NewUserRepository(db)
	profiles := repository.NewProfileRepository(db)
	swipes := repository.NewSwipeRepository(db)
	matches := repository.NewMatchRepository(db)

	createUser := func(email, name string, withPhoto bool) int64 {
		t.Helper()
		id, err := users.CreateUser(ctx, &model.UserInput{Email: email, PasswordHash: "test-hash"})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM "user" WHERE id=$1`, id) })
		birth := time.Now().AddDate(-25, -1, 0)
		if err := profiles.PatchProfile(ctx, id, &model.ProfilePatch{Name: &name, BirthDate: &birth}); err != nil {
			t.Fatal(err)
		}
		if withPhoto {
			for i := range 2 {
				if err := profiles.AddPhoto(ctx, id, fmt.Sprintf("match/%d-%d.jpg", id, i)); err != nil {
					t.Fatal(err)
				}
			}
		}
		return id
	}
	swipe := func(author, target int64, action model.SwipeAction) {
		t.Helper()
		if err := swipes.SaveSwipeAndMatch(ctx, &model.Swipe{AuthorID: author, TargetID: target, Action: action}); err != nil {
			t.Fatal(err)
		}
	}
	matchID := func(a, b int64) int64 {
		t.Helper()
		var id int64
		if err := db.QueryRowContext(ctx, `SELECT id FROM match WHERE first_user_id = LEAST($1::bigint, $2::bigint) AND second_user_id = GREATEST($1::bigint, $2::bigint)`, a, b).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	setPsycho := func(userID int64, score float64) {
		t.Helper()
		_, err := db.ExecContext(ctx, `
			INSERT INTO profile_psycho (profile_id, revision, openness, conscientiousness, extraversion, agreeableness, neuroticism)
			SELECT p.id, COALESCE((SELECT max(revision) FROM profile_psycho WHERE profile_id = p.id), 0) + 1, $2, $2, $2, $2, $2
			FROM profile AS p WHERE p.user_id = $1`, userID, score)
		if err != nil {
			t.Fatal(err)
		}
	}

	viewer := createUser("match-viewer@example.com", "Viewer", true)
	first := createUser("match-first@example.com", "First", true)
	second := createUser("match-second@example.com", "Second", false)
	oneSided := createUser("match-one-sided@example.com", "OneSided", true)
	stranger := createUser("match-stranger@example.com", "Stranger", true)

	setPsycho(first, 0.4)

	swipe(viewer, first, model.SwipeLike)
	swipe(first, viewer, model.SwipeSuperLike)
	swipe(second, viewer, model.SwipeLike)
	swipe(viewer, second, model.SwipeLike)
	swipe(viewer, oneSided, model.SwipeLike)
	swipe(stranger, first, model.SwipeLike)
	swipe(first, stranger, model.SwipeLike)
	firstMatch, secondMatch := matchID(viewer, first), matchID(viewer, second)
	if _, err := db.ExecContext(ctx, `INSERT INTO message (match_id, sender_id, body) VALUES ($1, $2, 'Привет')`, firstMatch, viewer); err != nil {
		t.Fatal(err)
	}

	all, total, next, err := matches.GetMatches(ctx, viewer, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || next != nil {
		t.Fatalf("total = %d, next cursor = %v; want 2, nil", total, next)
	}
	var ids []int64
	for _, m := range all {
		ids = append(ids, m.UserID)
	}
	if want := []int64{second, first}; !slices.Equal(ids, want) || all[0].ID != secondMatch || all[1].ID != firstMatch {
		t.Fatalf("matched users = %v, want %v", ids, want)
	}
	if m := all[1]; m.Name == nil || *m.Name != "First" || m.BirthDate == nil || m.PhotoKey == nil ||
		*m.PhotoKey != fmt.Sprintf("match/%d-0.jpg", first) || m.CreatedAt.IsZero() || m.IsNew ||
		m.Psycho.Openness == nil || *m.Psycho.Neuroticism != 0.4 {
		t.Fatalf("unexpected first match: %+v", m)
	}
	if m := all[0]; m.Name == nil || *m.Name != "Second" || m.PhotoKey != nil || !m.IsNew || m.Psycho.Openness != nil {
		t.Fatalf("unexpected second match: %+v", m)
	}

	page, total, next, err := matches.GetMatches(ctx, viewer, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(page) != 1 || page[0].UserID != second || next == nil || *next != page[0].ID {
		t.Fatalf("first page = %+v, next = %v", page, next)
	}
	page, _, next, err = matches.GetMatches(ctx, viewer, 1, next)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 1 || page[0].UserID != first || next != nil {
		t.Fatalf("second page = %+v, next = %v", page, next)
	}

	empty, total, next, err := matches.GetMatches(ctx, oneSided, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 || total != 0 || next != nil {
		t.Fatalf("one-sided like matches = %+v, next = %v", empty, next)
	}
}
