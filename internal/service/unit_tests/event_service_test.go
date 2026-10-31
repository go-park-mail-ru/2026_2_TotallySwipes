package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"dating-app/internal/model"
	. "dating-app/internal/service"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func eventSearchFixture() *model.PlaceSearch {
	return &model.PlaceSearch{SearchID: uuid.New(), MatchID: 42, City: "Москва", Status: model.SearchStatusVoting,
		ExpiresAt: time.Now().Add(time.Hour), Events: []model.Event{{ID: 2, Name: "second"}, {ID: 1, Name: "first"}},
		Participants: [2]model.ParticipantResult{{UserID: 10}, {UserID: 20}}}
}

func TestEventServiceCreate(t *testing.T) {
	failure := errors.New("dependency failed")
	for _, name := range []string{"new", "active", "active other city", "concurrent", "concurrent other city", "blank city", "access denied", "redis read error", "provider error", "empty", "save error"} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			provider, matches, repo := NewMockEventProvider(ctrl), NewMockMatchRepository(ctrl), NewMockEventRepository(ctrl)
			svc := NewEventService(provider, matches, repo)
			ctx := context.Background()
			city := " Москва "
			existing := eventSearchFixture()
			var wantErr error
			if name == "blank city" {
				city = " \t "
				wantErr = model.ErrInvalidCity
			} else {
				var accessErr error
				if name == "access denied" {
					accessErr = model.ErrNotFound
					wantErr = accessErr
				}
				access := matches.EXPECT().GetParticipants(ctx, int64(10), int64(42)).Return([2]int64{10, 20}, accessErr)
				if accessErr == nil {
					var active *model.PlaceSearch
					var readErr error
					if name == "active" || name == "active other city" {
						active = existing
					}
					if name == "active other city" {
						existing.City = "Казань"
						wantErr = model.ErrSearchAlreadyActive
					}
					if name == "redis read error" {
						readErr = failure
						wantErr = failure
					}
					read := repo.EXPECT().GetActive(ctx, int64(42)).After(access).Return(active, readErr)
					if active == nil && readErr == nil {
						page := &model.EventPage{Events: existing.Events}
						var providerErr error
						if name == "provider error" {
							providerErr = model.ErrTimepadTimeout
							wantErr = providerErr
						}
						if name == "empty" {
							page.Events = []model.Event{}
							wantErr = model.ErrNoEvents
						}
						get := provider.EXPECT().GetEvents(ctx, gomock.Any()).After(read).DoAndReturn(func(_ context.Context, q *model.EventQuery) (*model.EventPage, error) {
							if q.City == nil || *q.City != "Москва" || q.Skip == nil || *q.Skip != 0 || q.Limit == nil || *q.Limit != 10 || q.StartsAtMin == nil || q.StartsAtMin.IsZero() {
								t.Fatalf("bad query: %+v", q)
							}
							wantCategories := []int64{374, 376, 456, 457, 458, 459, 460, 461, 463, 524, 525, 2335}
							if q.CategoryIDs == nil || !reflect.DeepEqual(*q.CategoryIDs, wantCategories) {
								t.Fatal("missing category filter")
							}
							return page, providerErr
						})
						if providerErr == nil && len(page.Events) > 0 {
							if name == "save error" {
								wantErr = failure
							}
							if name == "concurrent other city" {
								existing.City = "Казань"
								wantErr = model.ErrSearchAlreadyActive
							}
							repo.EXPECT().CreateSearch(ctx, gomock.Any()).After(get).DoAndReturn(func(_ context.Context, s *model.PlaceSearch) (*model.PlaceSearch, bool, error) {
								if s.SearchID == uuid.Nil || s.MatchID != 42 || s.City != "Москва" || s.Participants[0].UserID != 10 || s.Participants[1].UserID != 20 || !reflect.DeepEqual(s.Events, page.Events) {
									t.Fatalf("bad candidate: %+v", s)
								}
								if name == "save error" {
									return nil, false, failure
								}
								if name == "concurrent" || name == "concurrent other city" {
									return existing, false, nil
								}
								return s, true, nil
							})
						}
					}
				}
			}
			state, created, err := svc.SearchEventsByCity(ctx, 10, 42, city)
			if !errors.Is(err, wantErr) {
				t.Fatalf("error=%v want=%v", err, wantErr)
			}
			if wantErr != nil {
				if state != nil || created {
					t.Fatal("success returned with error")
				}
				return
			}
			if state == nil || state.City != "Москва" || state.Status != model.SearchStatusVoting || created != (name == "new") {
				t.Fatalf("unexpected result: %+v created=%v", state, created)
			}
			if name != "new" && state.SearchID != existing.SearchID {
				t.Fatal("existing search replaced")
			}
		})
	}
}

func TestEventServiceGetState(t *testing.T) {
	for _, name := range []string{"voting", "waiting", "partner waiting", "completed", "no matches", "cancelled", "outsider", "wrong match", "missing", "access denied", "redis error"} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			provider, matches, repo := NewMockEventProvider(ctrl), NewMockMatchRepository(ctrl), NewMockEventRepository(ctrl)
			svc := NewEventService(provider, matches, repo)
			search := eventSearchFixture()
			id := search.SearchID
			user := int64(20)
			wantStatus := model.SearchStatusVoting
			var wantErr error
			switch name {
			case "waiting":
				search.Participants[1].Submitted = true
				search.Participants[1].LikedEventsIDs = []int64{}
				wantStatus = model.SearchStatusWaitingPartner
			case "partner waiting":
				search.Participants[0].Submitted = true
				search.Participants[0].LikedEventsIDs = []int64{1}
			case "completed", "no matches":
				search.Status = model.SearchStatusCompleted
				wantStatus = search.Status
				search.Participants[0].Submitted = true
				search.Participants[1].Submitted = true
				search.Participants[0].LikedEventsIDs = []int64{1, 2}
				search.Participants[1].LikedEventsIDs = []int64{2, 1}
				if name == "no matches" {
					search.Participants[1].LikedEventsIDs = []int64{}
				}
			case "cancelled":
				search.Status = model.SearchStatusCancelled
				wantStatus = search.Status
			case "outsider":
				user = 30
				wantErr = model.ErrNotFound
			case "wrong match":
				search.MatchID = 99
				wantErr = model.ErrNotFound
			case "missing":
				search = nil
				wantErr = model.ErrNotFound
			case "access denied":
				wantErr = model.ErrNotFound
			case "redis error":
				wantErr = errors.New("redis unavailable")
			}
			var accessErr error
			if name == "access denied" {
				accessErr = wantErr
			}
			matches.EXPECT().GetParticipants(gomock.Any(), user, int64(42)).Return([2]int64{10, 20}, accessErr)
			if accessErr == nil {
				var readErr error
				if name == "redis error" {
					readErr = wantErr
				}
				repo.EXPECT().GetBySearchID(gomock.Any(), id).Return(search, readErr)
			}
			state, err := svc.GetSearchEvents(context.Background(), user, 42, id)
			if !errors.Is(err, wantErr) {
				t.Fatalf("error=%v want=%v", err, wantErr)
			}
			if wantErr != nil {
				return
			}
			if state.Status != wantStatus || state.CurrentUserFinished != search.Participants[1].Submitted || state.PartnerFinished != search.Participants[0].Submitted || !state.ExpiresAt.Equal(search.ExpiresAt) {
				t.Fatalf("bad perspective: %+v", state)
			}
			if state.Matches == nil {
				t.Fatal("matches must not be nil")
			}
			if !state.CurrentUserFinished && state.CurrentUserLikedEventIDs != nil {
				t.Fatal("unsubmitted likes exposed")
			}
			if state.CurrentUserFinished && state.CurrentUserLikedEventIDs == nil {
				t.Fatal("submitted empty likes must be an array")
			}
			if name == "completed" {
				if !reflect.DeepEqual(state.Matches, search.Events) || !reflect.DeepEqual(state.CurrentUserLikedEventIDs, []int64{2, 1}) {
					t.Fatal("intersection order or likes incorrect")
				}
				state.CurrentUserLikedEventIDs[0] = 999
				if search.Participants[1].LikedEventsIDs[0] != 2 {
					t.Fatal("response aliases stored likes")
				}
			} else if len(state.Matches) != 0 {
				t.Fatal("unexpected intersection")
			}
			if name == "waiting" && search.Status != model.SearchStatusVoting {
				t.Fatal("mapping mutated shared status")
			}
		})
	}
}

func TestEventServiceSubmit(t *testing.T) {
	for _, name := range []string{"first", "empty", "completed", "duplicate IDs", "unknown ID", "already submitted", "state conflict", "redis error", "access denied"} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			provider, matches, repo := NewMockEventProvider(ctrl), NewMockMatchRepository(ctrl), NewMockEventRepository(ctrl)
			svc := NewEventService(provider, matches, repo)
			search := eventSearchFixture()
			likes := []int64{1}
			var wantErr error
			switch name {
			case "empty":
				likes = []int64{}
			case "duplicate IDs":
				likes = []int64{1, 1}
				wantErr = model.ErrInvalidEventResults
			case "unknown ID":
				likes = []int64{99}
				wantErr = model.ErrInvalidEventResults
			case "already submitted":
				wantErr = model.ErrResultAlreadySubmitted
			case "state conflict":
				wantErr = model.ErrSearchStateConflict
			case "redis error":
				wantErr = errors.New("redis down")
			case "access denied":
				wantErr = model.ErrNotFound
			}
			var accessErr error
			if name == "access denied" {
				accessErr = wantErr
			}
			matches.EXPECT().GetParticipants(gomock.Any(), int64(20), int64(42)).Return([2]int64{10, 20}, accessErr)
			if accessErr == nil {
				get := repo.EXPECT().GetBySearchID(gomock.Any(), search.SearchID).Return(search, nil)
				if !errors.Is(wantErr, model.ErrInvalidEventResults) {
					result := *search
					result.Participants[1].Submitted = true
					result.Participants[1].LikedEventsIDs = likes
					if name == "completed" {
						result.Participants[0].Submitted = true
						result.Participants[0].LikedEventsIDs = []int64{1}
						result.Status = model.SearchStatusCompleted
					}
					repo.EXPECT().SubmitResults(gomock.Any(), int64(20), int64(42), search.SearchID, likes).After(get).Return(&result, wantErr)
				}
			}
			state, err := svc.SubmitResults(context.Background(), 20, 42, search.SearchID, likes)
			if !errors.Is(err, wantErr) {
				t.Fatalf("error=%v want=%v", err, wantErr)
			}
			if wantErr != nil {
				return
			}
			if !state.CurrentUserFinished || !reflect.DeepEqual(state.CurrentUserLikedEventIDs, likes) {
				t.Fatalf("bad result: %+v", state)
			}
			if name == "completed" {
				if state.Status != model.SearchStatusCompleted || len(state.Matches) != 1 || state.Matches[0].ID != 1 {
					t.Fatal("missing completed intersection")
				}
			} else if state.Status != model.SearchStatusWaitingPartner {
				t.Fatal("not waiting")
			}
		})
	}
}
