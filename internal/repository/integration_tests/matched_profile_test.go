package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"dating-app/internal/model"
	"dating-app/internal/repository"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestProfileRepoGetMatchedProfile(t *testing.T) {
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

	createUser := func(email, name string) int64 {
		t.Helper()
		id, err := users.CreateUser(ctx, &model.UserInput{Email: email, PasswordHash: "test-hash"})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM "user" WHERE id=$1`, id) })
		birth := time.Now().AddDate(-24, -1, 0)
		about, work, tags := "Работаю в издательстве", "Редактор", []string{"books", "bicycle"}
		patch := model.ProfilePatch{Name: &name, BirthDate: &birth, AboutMe: model.Optional[string]{Set: true, Value: &about},
			Work: model.Optional[string]{Set: true, Value: &work}, Tags: &tags}
		if err := profiles.PatchProfile(ctx, id, &patch); err != nil {
			t.Fatal(err)
		}
		if err := profiles.AddPhoto(ctx, id, fmt.Sprintf("view/%d.jpg", id)); err != nil {
			t.Fatal(err)
		}
		return id
	}
	swipe := func(author, target int64) {
		t.Helper()
		if err := swipes.SaveSwipeAndMatch(ctx, &model.Swipe{AuthorID: author, TargetID: target, Action: model.SwipeLike}); err != nil {
			t.Fatal(err)
		}
	}

	viewer := createUser("view-viewer@example.com", "Илья")
	matched := createUser("view-matched@example.com", "Даша")
	liked := createUser("view-liked@example.com", "Соня")
	stranger := createUser("view-stranger@example.com", "Вера")
	swipe(viewer, matched)
	swipe(matched, viewer)
	swipe(viewer, liked)
	swipe(stranger, viewer)

	p, matchID, err := profiles.GetMatchedProfile(ctx, viewer, matched)
	if err != nil {
		t.Fatal(err)
	}
	f := p.CurrentVersion.ProfileFields
	if matchID <= 0 || p.UserID != matched || f.Name == nil || *f.Name != "Даша" || f.Work == nil || *f.Work != "Редактор" ||
		len(p.Tags) != 2 || len(p.Photos) != 1 || p.Photos[0].StorageKey != fmt.Sprintf("view/%d.jpg", matched) {
		t.Fatalf("matched profile = %+v, match id = %d", p, matchID)
	}
	if _, back, err := profiles.GetMatchedProfile(ctx, matched, viewer); err != nil || back != matchID {
		t.Fatalf("reverse lookup: match id = %d, err = %v; want %d", back, err, matchID)
	}

	for name, target := range map[string]int64{"own like only": liked, "their like only": stranger, "missing user": 1 << 40} {
		if _, _, err := profiles.GetMatchedProfile(ctx, viewer, target); !errors.Is(err, model.ErrNotFound) {
			t.Fatalf("%s: err = %v, want ErrNotFound", name, err)
		}
	}
}
