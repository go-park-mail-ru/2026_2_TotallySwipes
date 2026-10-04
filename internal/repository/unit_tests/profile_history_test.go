package repository_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"dating-app/internal/model"
	"dating-app/internal/repository"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Run against a disposable database initialized with db/migrations/000001_init.up.sql.
func TestProfileTagHistory(t *testing.T) {
	dsn := os.Getenv("TEST_HISTORY_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_HISTORY_DATABASE_URL is not set")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	repo := repository.NewProfileRepository(db)
	users := repository.NewAuthUserRepository(db)
	version := model.ProfileVersionInput{BirthDate: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), Sex: model.SexFemale, SearchSex: model.SearchSexAll, SearchAgeFrom: 18, SearchAgeTo: 100, DatingGoal: model.DatingGoalFriendship}
	userID, err := users.CreateUserWithProfile(ctx, &model.UserInput{Name: "History", Email: "history-test@example.com", PasswordHash: "test-hash"}, &version, []string{"history-a", "history-b"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(ctx, `DELETE FROM "user" WHERE id=$1`, userID)
	first, err := repo.GetByUserIDCurrentProfile(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Tags) != 2 {
		t.Fatalf("initial tags: %+v", first.Tags)
	}
	version.Tags = []model.Tag{first.Tags[1]}
	if err := repo.AddProfileVersion(ctx, first.ID, &version); err != nil {
		t.Fatal(err)
	}
	second, err := repo.GetByUserIDCurrentProfile(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if second.CurrentVersion.ID == first.CurrentVersion.ID || len(second.Tags) != 1 || second.Tags[0].ID != first.Tags[1].ID {
		t.Fatalf("new snapshot: %+v", second)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM profile_tag WHERE profile_version_id=$1`, first.CurrentVersion.ID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("old tags lost: %d %v", count, err)
	}
	// A foreign-key failure must roll back the new version as well as its tags.
	version.Tags = []model.Tag{{ID: -1}}
	if err := repo.AddProfileVersion(ctx, first.ID, &version); err == nil {
		t.Fatal("invalid tag accepted")
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM profile_version WHERE profile_id=$1`, first.ID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("failed version not rolled back: %d %v", count, err)
	}
	// A full replacement with no tags must not resurrect tags from older versions.
	version.Tags = nil
	if err := repo.AddProfileVersion(ctx, first.ID, &version); err != nil {
		t.Fatal(err)
	}
	current, err := repo.GetByUserIDCurrentProfile(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(current.Tags) != 0 {
		t.Fatal("empty version inherited old tags")
	}
	feed, _, err := repo.GetProfilesByCursorAndLimit(ctx, 0, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range feed {
		if p.ID == first.ID {
			found = true
			if len(p.Tags) != 0 {
				t.Fatal("feed contains historical tags")
			}
		}
	}
	if !found {
		t.Fatal("profile missing from feed")
	}
}
