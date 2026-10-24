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
	locations := repository.NewLocationRepository(db)
	city := fmt.Sprintf("feed-city-%d", time.Now().UnixNano())
	setCity := func(id int64, city string) {
		t.Helper()
		if err := locations.Upsert(ctx, &model.UserLocation{UserID: id, City: city, Coordinates: model.Coordinates{Latitude: 55.75, Longitude: 37.61}}); err != nil {
			t.Fatal(err)
		}
	}

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
		setCity(id, city)
		return id
	}

	viewer := createUser("filter-viewer@example.com", model.SexMale, 30)
	otherCity := createUser("filter-other-city@example.com", model.SexFemale, 25)
	setCity(otherCity, city+"-other")
	noLocation := createUser("filter-no-location@example.com", model.SexFemale, 25)
	if _, err := db.ExecContext(ctx, `DELETE FROM user_location WHERE user_id=$1`, noLocation); err != nil {
		t.Fatal(err)
	}
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
	candidates := []int64{female25, male25, female50, noFilter, otherCity, noLocation}

	feedIDs := func() []int64 {
		t.Helper()
		page, _, err := profiles.GetCandidatesByCursorAndLimit(ctx, viewer, 100, nil)
		if err != nil {
			t.Fatal(err)
		}
		var ids []int64
		for _, p := range page {
			if p.Profile.City == nil || *p.Profile.City != city {
				t.Fatalf("wrong candidate city: %+v", p.Profile)
			}
			if slices.Contains(candidates, p.Profile.UserID) {
				ids = append(ids, p.Profile.UserID)
			}
		}
		return ids
	}

	if got, want := feedIDs(), candidates[:3]; !slices.Equal(got, want) {
		t.Fatalf("feed with default filter = %v, want %v", got, want)
	}

	setSwipe := func(author, target int64, action string) {
		t.Helper()
		_, err := db.ExecContext(ctx, `
			INSERT INTO swipe (author_id, target_profile_id, action)
			SELECT $1, id, $3 FROM profile WHERE user_id = $2
			ON CONFLICT (author_id, target_profile_id)
			DO UPDATE SET action = EXCLUDED.action, updated_at = CURRENT_TIMESTAMP`, author, target, action)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Исходящий лайк и лайк кандидата другому пользователю не означают liked_me.
	setSwipe(viewer, female25, "like")
	setSwipe(female25, male25, "like")
	for _, tc := range []struct {
		name, action string
		want         bool
	}{
		{"no incoming swipe", "", false},
		{"incoming like", "like", true},
		{"incoming dislike replaces like", "dislike", false},
		{"incoming super like", "super_like", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.action != "" {
				setSwipe(female25, viewer, tc.action)
			}
			cursor := female25 - 1
			page, next, err := profiles.GetCandidatesByCursorAndLimit(ctx, viewer, 1, &cursor)
			if err != nil {
				t.Fatal(err)
			}
			if len(page) != 1 || page[0].Profile.UserID != female25 || page[0].LikedMe != tc.want {
				t.Fatalf("page = %+v, want user %d with liked_me=%v", page, female25, tc.want)
			}
			if next == nil || *next != female25 {
				t.Fatalf("next cursor = %v, want %d", next, female25)
			}
		})
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
	if err := filters.Upsert(ctx, viewer, &model.DefaultSearchFilter); err != nil {
		t.Fatal(err)
	}
	// Кандидаты другого города/без локации созданы раньше: они не должны занимать LIMIT.
	var cursor *int64
	for i, wantID := range candidates[:3] {
		page, next, err := profiles.GetCandidatesByCursorAndLimit(ctx, viewer, 1, cursor)
		if err != nil || len(page) != 1 || page[0].Profile.UserID != wantID {
			t.Fatalf("page %d: %+v, %v", i, page, err)
		}
		if i < 2 && (next == nil || *next != wantID) {
			t.Fatalf("page %d cursor: %v", i, next)
		}
		if i == 2 && next != nil {
			t.Fatalf("last page cursor: %v", next)
		}
		cursor = next
	}
	after := female50
	page, next, err := profiles.GetCandidatesByCursorAndLimit(ctx, viewer, 1, &after)
	if err != nil || len(page) != 0 || next != nil {
		t.Fatalf("empty page: %+v, %v, %v", page, next, err)
	}
	setCity(viewer, city+"-other")
	page, next, err = profiles.GetCandidatesByCursorAndLimit(ctx, viewer, 10, nil)
	if err != nil || len(page) != 1 || page[0].Profile.UserID != otherCity || next != nil {
		t.Fatalf("changed city: %+v, %v, %v", page, next, err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM user_location WHERE user_id=$1`, viewer); err != nil {
		t.Fatal(err)
	}
	page, next, err = profiles.GetCandidatesByCursorAndLimit(ctx, viewer, 10, nil)
	if err != nil || len(page) != 0 || next != nil {
		t.Fatalf("viewer without location: %+v, %v, %v", page, next, err)
	}

}
