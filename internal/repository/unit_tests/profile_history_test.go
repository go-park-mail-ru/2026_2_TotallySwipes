package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"testing"
	"time"

	"dating-app/internal/model"
	"dating-app/internal/repository"
	_ "github.com/jackc/pgx/v5/stdlib"
)

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
	users := repository.NewUserRepository(db)

	userID, err := users.CreateUser(ctx, &model.UserInput{Email: "history-test@example.com", PasswordHash: "test-hash"})
	if err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(ctx, `DELETE FROM "user" WHERE id=$1`, userID)

	empty, err := repo.GetByUserIDCurrentProfile(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if empty.CurrentVersion.ID != 0 || len(empty.Missing()) != 6 {
		t.Fatalf("new profile must be empty: %+v", empty)
	}

	name, birth := "History", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	sex, goal := model.SexFemale, model.DatingGoalFriendship
	education, work, smoking, height := model.EducationHigher, "Инженер", model.AttitudeNegative, 180
	tags := []string{"coffee", "music"}
	steps := []model.ProfilePatch{
		{Name: &name, BirthDate: &birth, Sex: &sex},
		{DatingGoal: &goal, Education: model.Optional[model.Education]{Set: true, Value: &education},
			Work: model.Optional[string]{Set: true, Value: &work}, Smoking: model.Optional[model.Attitude]{Set: true, Value: &smoking},
			Height: model.Optional[int]{Set: true, Value: &height}},
		{Tags: &tags},
	}
	for _, step := range steps {
		if err := repo.PatchProfile(ctx, userID, &step); err != nil {
			t.Fatal(err)
		}
	}
	if err := repository.NewFilterRepository(db).Upsert(ctx, userID, &model.DefaultSearchFilter); err != nil {
		t.Fatal(err)
	}
	first, err := repo.GetByUserIDCurrentProfile(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if *first.CurrentVersion.Name != name || len(first.Tags) != 2 || !reflect.DeepEqual(first.Missing(), []string{"photos"}) {
		t.Fatalf("after steps: %+v missing=%v", first, first.Missing())
	}

	details := first.CurrentVersion.ProfileFields
	if *details.Education != education || *details.Work != work || *details.Smoking != smoking || details.Alcohol != nil || *details.Height != height {
		t.Fatalf("details: %+v", details)
	}

	newName := "Renamed"
	if err := repo.PatchProfile(ctx, userID, &model.ProfilePatch{Name: &newName}); err != nil {
		t.Fatal(err)
	}
	second, err := repo.GetByUserIDCurrentProfile(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if second.CurrentVersion.ID == first.CurrentVersion.ID || len(second.Tags) != 2 || *second.CurrentVersion.Sex != sex ||
		*second.CurrentVersion.Height != height {
		t.Fatalf("new snapshot: %+v", second)
	}

	unknown := []string{"no-such-tag"}
	if err := repo.PatchProfile(ctx, userID, &model.ProfilePatch{Tags: &unknown}); !errors.Is(err, model.ErrUnknownTag) {
		t.Fatalf("unknown tag: %v", err)
	}
	var versions int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM profile_version WHERE profile_id=$1`, first.ID).Scan(&versions); err != nil || versions != 4 {
		t.Fatalf("failed version not rolled back: %d %v", versions, err)
	}

	for i := 1; i <= model.MaxPhotos; i++ {
		if err := repo.AddPhoto(ctx, userID, fmt.Sprintf("history/%d.jpg", i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.AddPhoto(ctx, userID, "history/extra.jpg"); !errors.Is(err, model.ErrPhotoLimit) {
		t.Fatalf("photo limit: %v", err)
	}
	withPhotos, err := repo.GetByUserIDCurrentProfile(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]int64, 0, len(withPhotos.Photos))
	for _, photo := range withPhotos.Photos {
		ids = append(ids, photo.ID)
	}
	reversed := slices.Clone(ids)
	slices.Reverse(reversed)
	if err := repo.ReorderPhotos(ctx, userID, reversed); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReorderPhotos(ctx, userID, reversed[1:]); !errors.Is(err, model.ErrPhotoOrderMismatch) {
		t.Fatalf("partial order: %v", err)
	}
	if err := repo.ReorderPhotos(ctx, userID, append(slices.Clone(reversed[1:]), 999999)); !errors.Is(err, model.ErrPhotoOrderMismatch) {
		t.Fatalf("foreign photo: %v", err)
	}
	reordered, err := repo.GetByUserIDCurrentProfile(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	for i, photo := range reordered.Photos {
		if photo.ID != reversed[i] || photo.Position != i+1 {
			t.Fatalf("photos not reordered: %+v", reordered.Photos)
		}
	}
	if err := repo.ReorderPhotos(ctx, userID, ids); err != nil {
		t.Fatal(err)
	}

	if key, err := repo.DeletePhoto(ctx, userID, withPhotos.Photos[0].ID); err != nil || key != "history/1.jpg" {
		t.Fatalf("delete main photo: %q %v", key, err)
	}
	shifted, err := repo.GetByUserIDCurrentProfile(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	for i, photo := range shifted.Photos {
		if photo.Position != i+1 || photo.StorageKey != fmt.Sprintf("history/%d.jpg", i+2) {
			t.Fatalf("photos not shifted: %+v", shifted.Photos)
		}
	}
	for _, photo := range shifted.Photos[:len(shifted.Photos)-1] {
		if _, err := repo.DeletePhoto(ctx, userID, photo.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.DeletePhoto(ctx, userID, shifted.Photos[len(shifted.Photos)-1].ID); !errors.Is(err, model.ErrLastPhoto) {
		t.Fatalf("last photo: %v", err)
	}

	feed, _, err := repo.GetProfilesByCursorAndLimit(ctx, 0, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range feed {
		found = found || p.ID == first.ID
	}
	if !found {
		t.Fatal("filled profile missing from feed")
	}
}
