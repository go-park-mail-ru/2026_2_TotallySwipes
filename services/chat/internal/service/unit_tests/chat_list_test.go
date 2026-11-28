package service_test

import (
	"context"
	"testing"

	"dating-app/services/chat/internal/model"
	"go.uber.org/mock/gomock"
)

func TestGetChatsDefaults(t *testing.T) {

	f := newFixture(t)
	expected := &model.ChatPage{Items: []model.ChatCard{}}
	f.repo.EXPECT().GetChats(gomock.Any(), int64(1), model.ChatListQuery{Limit: ptr(100), Cursor: ptr("0")}).Return(expected, nil)

	page, err := f.svc.GetChats(context.Background(), 1, model.ChatListQuery{})
	if err != nil || page != expected {
		t.Fatalf("page=%v error=%v", page, err)
	}
}

func TestGetChatsValidation(t *testing.T) {

	for _, query := range []model.ChatListQuery{
		{Limit: ptr(0)}, {Limit: ptr(-1)}, {Limit: ptr(101)},
		{Cursor: ptr("")}, {Cursor: ptr("-1")}, {Cursor: ptr("bad")},
	} {
		f := newFixture(t)
		_, err := f.svc.GetChats(context.Background(), 1, query)
		assertError(t, err, model.ErrInvalidChatsQuery)
	}

	f := newFixture(t)
	_, err := f.svc.GetChats(context.Background(), 0, model.ChatListQuery{})
	assertError(t, err, model.ErrInvalidUserID)
}

func TestGetChatsProfiles(t *testing.T) {

	f := newFixture(t)
	next := "100"
	stored := &model.ChatPage{
		Items: []model.ChatCard{
			{ChatID: 10, Peer: model.ChatPeer{UserID: 2}},
			{ChatID: 11, Peer: model.ChatPeer{UserID: 3}},
			{ChatID: 12, Peer: model.ChatPeer{UserID: 2}},
		},
		NextCursor: &next,
	}
	profile := model.ChatProfile{Name: ptr("Анна"), Age: ptr(25), AvatarURL: ptr("https://example.com/avatar.jpg")}

	gomock.InOrder(
		f.repo.EXPECT().GetChats(gomock.Any(), int64(1), gomock.Any()).Return(stored, nil),
		f.profile.EXPECT().GetChatProfiles(gomock.Any(), []int64{2, 3}).Return(map[int64]model.ChatProfile{2: profile}, nil),
	)

	page, err := f.svc.GetChats(context.Background(), 1, model.ChatListQuery{})
	if err != nil {
		t.Fatal(err)
	}

	if page.NextCursor == nil || *page.NextCursor != next || len(page.Items) != 3 {
		t.Fatalf("unexpected page: %+v", page)
	}

	for _, index := range []int{0, 2} {
		peer := page.Items[index].Peer
		if peer.UserID != 2 || peer.Name == nil || *peer.Name != "Анна" || peer.Age == nil || *peer.Age != 25 || peer.AvatarURL == nil || *peer.AvatarURL != *profile.AvatarURL {
			t.Fatalf("unexpected peer: %+v", peer)
		}
	}

	if peer := page.Items[1].Peer; peer.UserID != 3 || peer.Name != nil {
		t.Fatalf("missing profile changed peer identity: %+v", peer)
	}
}

func TestGetChatsDependencyErrors(t *testing.T) {

	t.Run("repository", func(t *testing.T) {
		f := newFixture(t)
		f.repo.EXPECT().GetChats(gomock.Any(), int64(1), gomock.Any()).Return(nil, dependencyErr)

		page, err := f.svc.GetChats(context.Background(), 1, model.ChatListQuery{})
		assertError(t, err, dependencyErr)
		if page != nil {
			t.Fatal("unexpected page on error")
		}
	})

	t.Run("profile", func(t *testing.T) {
		f := newFixture(t)
		stored := &model.ChatPage{Items: []model.ChatCard{{Peer: model.ChatPeer{UserID: 2}}}}
		f.repo.EXPECT().GetChats(gomock.Any(), int64(1), gomock.Any()).Return(stored, nil)
		f.profile.EXPECT().GetChatProfiles(gomock.Any(), []int64{2}).Return(nil, dependencyErr)

		page, err := f.svc.GetChats(context.Background(), 1, model.ChatListQuery{})
		assertError(t, err, dependencyErr)
		if page != nil {
			t.Fatal("unexpected page on error")
		}
	})
}
