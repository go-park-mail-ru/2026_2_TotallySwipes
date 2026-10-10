package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"dating-app/internal/model"
	"dating-app/internal/repository"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestSearchFilterFeed(t *testing.T) {
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
	filters := repository.NewFilterRepository(db)

	createUser := func(email string, sex model.Sex, age int) int64 {
		t.Helper()
		id, err := users.CreateUser(ctx, &model.UserInput{Email: email, PasswordHash: "test-hash"})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM "user" WHERE id=$1`, id) })

		name, goal := "Filter", model.DatingGoalFriendship
		birth := time.Now().AddDate(-age, -1, 0)
		patch := model.ProfilePatch{Name: &name, BirthDate: &birth, Sex: &sex, DatingGoal: &goal}
		if err := profiles.PatchProfile(ctx, id, &patch); err != nil {
			t.Fatal(err)
		}
		if err := profiles.AddPhoto(ctx, id, fmt.Sprintf("filter/%d.jpg", id)); err != nil {
			t.Fatal(err)
		}
		if err := filters.Upsert(ctx, id, &model.DefaultSearchFilter); err != nil {
			t.Fatal(err)
		}
		return id
	}

	viewer := createUser("filter-viewer@example.com", model.SexMale, 30)
	female25 := createUser("filter-f25@example.com", model.SexFemale, 25)
	male25 := createUser("filter-m25@example.com", model.SexMale, 25)
	female50 := createUser("filter-f50@example.com", model.SexFemale, 50)
	noFilter := createUser("filter-none@example.com", model.SexFemale, 25)
	if _, err := db.ExecContext(ctx, `DELETE FROM search_filter WHERE user_id=$1`, noFilter); err != nil {
		t.Fatal(err)
	}
	if _, err := filters.Get(ctx, noFilter); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("deleted filter: %v", err)
	}
	candidates := []int64{female25, male25, female50, noFilter}

	feedIDs := func() []int64 {
		t.Helper()
		page, _, err := profiles.GetProfilesByCursorAndLimit(ctx, viewer, 100, nil)
		if err != nil {
			t.Fatal(err)
		}
		var ids []int64
		for _, p := range page {
			if slices.Contains(candidates, p.UserID) {
				ids = append(ids, p.UserID)
			}
		}
		return ids
	}

	if got, want := feedIDs(), candidates[:3]; !slices.Equal(got, want) {
		t.Fatalf("feed with default filter = %v, want %v", got, want)
	}

	for _, tc := range []struct {
		filter model.SearchFilter
		want   []int64
	}{
		{model.SearchFilter{Sex: model.SearchSexFemale, AgeFrom: 20, AgeTo: 30}, []int64{female25}},
		{model.SearchFilter{Sex: model.SearchSexAll, AgeFrom: 25, AgeTo: 25}, []int64{female25, male25}},
		{model.SearchFilter{Sex: model.SearchSexFemale, AgeFrom: 18, AgeTo: 100}, []int64{female25, female50}},
	} {
		if err := filters.Upsert(ctx, viewer, &tc.filter); err != nil {
			t.Fatal(err)
		}
		saved, err := filters.Get(ctx, viewer)
		if err != nil || *saved != tc.filter {
			t.Fatalf("saved filter = %+v, %v; want %+v", saved, err, tc.filter)
		}
		if got := feedIDs(); !slices.Equal(got, tc.want) {
			t.Fatalf("feed with %+v = %v, want %v", tc.filter, got, tc.want)
		}
	}
}
