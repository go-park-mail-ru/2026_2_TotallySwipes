package service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"dating-app/services/chat/internal/model"
	"go.uber.org/mock/gomock"
)

func mediaCommand() model.ChatSendMedia {
	return model.ChatSendMedia{ChatID: 10, ClientMessageID: "key", Type: model.ChatMessageTypeVoice, Data: []byte("audio bytes")}
}

func mediaDigest(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func storedMedia(key string) *model.StoredMessage {
	return &model.StoredMessage{ID: 100, ChatID: 10, ClientMessageID: ptr("key"), Seq: 5, SenderID: ptr(int64(1)), Type: model.ChatMessageTypeVoice, StorageKey: &key, DurationSeconds: ptr(2.5), CreatedAt: now, Version: 1}
}

func expectMediaAccess(f fixture) {
	f.repo.EXPECT().GetChatState(gomock.Any(), int64(1), int64(10)).Return(&model.ChatState{MatchExists: true, MatchAllowsMessaging: true}, nil)
}

func TestSendMediaValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		userID int64
		change func(*model.ChatSendMedia)
		want   error
	}{
		{"пользователь", 0, nil, model.ErrInvalidUserID},
		{"чат", 1, func(c *model.ChatSendMedia) { c.ChatID = 0 }, model.ErrInvalidChatID},
		{"ключ", 1, func(c *model.ChatSendMedia) { c.ClientMessageID = " \n" }, model.ErrInvalidClientMessageID},
		{"длинный ключ", 1, func(c *model.ChatSendMedia) { c.ClientMessageID = strings.Repeat("я", 129) }, model.ErrInvalidClientMessageID},
		{"кодировка ключа", 1, func(c *model.ChatSendMedia) { c.ClientMessageID = string([]byte{255}) }, model.ErrInvalidClientMessageID},
		{"тип", 1, func(c *model.ChatSendMedia) { c.Type = model.ChatMessageTypeText }, model.ErrInvalidMedia},
		{"пустой файл", 1, func(c *model.ChatSendMedia) { c.Data = nil }, model.ErrInvalidMedia},
		{"размер голоса", 1, func(c *model.ChatSendMedia) { c.Data = make([]byte, (5<<20)+1) }, model.ErrMediaTooLarge},
		{"размер видео", 1, func(c *model.ChatSendMedia) {
			c.Type = model.ChatMessageTypeVideoNote
			c.Data = make([]byte, (10<<20)+1)
		}, model.ErrMediaTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			command := mediaCommand()
			if tc.change != nil {
				tc.change(&command)
			}
			message, created, err := f.svc.SendMedia(context.Background(), tc.userID, command)
			assertError(t, err, tc.want)
			if message != nil || created {
				t.Fatalf("message = %v, created = %v", message, created)
			}
		})
	}
}

func TestSendMediaAccess(t *testing.T) {
	for _, name := range []string{"нет чата", "отключён отправитель", "отключён собеседник", "нет мэтча", "закрыта переписка", "блокировка пары", "ошибка поиска"} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			command := mediaCommand()
			state := &model.ChatState{MatchExists: true, MatchAllowsMessaging: true}
			stateErr, wantErr := error(nil), model.ErrMessagingNotAllowed
			switch name {
			case "нет чата":
				stateErr, wantErr = model.ErrChatNotFound, model.ErrChatNotFound
			case "отключён отправитель":
				state.ViewerDisabled = true
				wantErr = model.ErrChatMemberBlocked
			case "отключён собеседник":
				state.PeerDisabled = true
			case "нет мэтча":
				state.MatchExists = false
			case "закрыта переписка":
				state.MatchAllowsMessaging = false
			case "блокировка пары":
				state.PeerBlockedViewer = true
			}
			f.repo.EXPECT().GetChatState(gomock.Any(), int64(1), int64(10)).Return(state, stateErr)
			if stateErr == nil && !state.ViewerDisabled {
				lookupErr := model.ErrMessageNotFound
				if name == "ошибка поиска" {
					lookupErr, wantErr = dependencyErr, dependencyErr
				}
				f.repo.EXPECT().GetMediaByClientID(gomock.Any(), int64(1), int64(10), "key").Return(nil, lookupErr)
			}
			_, _, err := f.svc.SendMedia(context.Background(), 1, command)
			assertError(t, err, wantErr)
		})
	}
}

func TestSendMediaRetry(t *testing.T) {
	for _, name := range []string{"повтор", "другой файл", "другой тип"} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			command := mediaCommand()
			expectMediaAccess(f)
			stored := storedMedia("voice/existing")
			digest := mediaDigest(command.Data)
			if name == "другой файл" {
				digest = "different"
			}
			if name == "другой тип" {
				stored.Type = model.ChatMessageTypeVideoNote
			}
			f.repo.EXPECT().GetMediaByClientID(gomock.Any(), int64(1), int64(10), "key").Return(&model.StoredMediaMessage{Message: *stored, SHA256: digest}, nil)
			if name == "повтор" {
				f.links.EXPECT().BuildMessageMediaURL(int64(10), int64(100)).Return("/media")
			}
			got, created, err := f.svc.SendMedia(context.Background(), 1, command)
			wantErr := error(nil)
			if name != "повтор" {
				wantErr = model.ErrIdempotencyConflict
			}
			assertError(t, err, wantErr)
			if created || (err == nil && got.Attachment.URL != "/media") {
				t.Fatalf("message = %v, created = %v", got, created)
			}
		})
	}
}

func TestSendMediaInspection(t *testing.T) {
	for _, tc := range []struct {
		name              string
		info              *model.MediaInfo
		probeErr, wantErr error
	}{
		{"ошибка инспектора", nil, dependencyErr, dependencyErr},
		{"неизвестный формат", nil, model.ErrUnsupportedMedia, model.ErrUnsupportedMedia},
		{"нет результата", nil, nil, model.ErrInvalidMedia},
		{"нулевая длительность", &model.MediaInfo{}, nil, model.ErrInvalidMedia},
		{"NaN", &model.MediaInfo{DurationSeconds: math.NaN()}, nil, model.ErrInvalidMedia},
		{"бесконечность", &model.MediaInfo{DurationSeconds: math.Inf(1)}, nil, model.ErrInvalidMedia},
		{"длинный голос", &model.MediaInfo{DurationSeconds: 301}, nil, model.ErrMediaTooLong},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			command := mediaCommand()
			expectMediaAccess(f)
			f.repo.EXPECT().GetMediaByClientID(gomock.Any(), int64(1), int64(10), "key").Return(nil, model.ErrMessageNotFound)
			f.inspector.EXPECT().Inspect(gomock.Any(), command.Data, command.Type).Return(tc.info, tc.probeErr)
			_, _, err := f.svc.SendMedia(context.Background(), 1, command)
			assertError(t, err, tc.wantErr)
		})
	}
}

func TestSendMediaSave(t *testing.T) {
	for _, name := range []string{"новое", "видеокружок", "ошибка MinIO", "откат SQL", "ошибка удаления", "параллельный повтор", "параллельный конфликт", "commit подтверждён", "commit не выяснен", "commit другая отправка"} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			command := mediaCommand()
			contentType := "audio/webm"
			if name == "видеокружок" {
				command.Type, contentType = model.ChatMessageTypeVideoNote, "video/webm"
			}
			expectMediaAccess(f)
			f.repo.EXPECT().GetMediaByClientID(gomock.Any(), int64(1), int64(10), "key").Return(nil, model.ErrMessageNotFound)
			f.inspector.EXPECT().Inspect(gomock.Any(), command.Data, command.Type).Return(&model.MediaInfo{DurationSeconds: 2.5, ContentType: contentType}, nil)
			saveErr := error(nil)
			if name == "ошибка MinIO" {
				saveErr = dependencyErr
			}
			f.storage.EXPECT().Save(gomock.Any(), command.Data, command.Type, contentType).Return("voice/new", saveErr)
			wantErr := error(nil)
			created := name == "новое" || name == "видеокружок" || name == "commit подтверждён"
			stored := storedMedia("voice/new")
			stored.Type = command.Type
			if saveErr != nil {
				wantErr = saveErr
			} else {
				createErr := error(nil)
				switch name {
				case "откат SQL", "ошибка удаления":
					createErr, wantErr = dependencyErr, dependencyErr
				case "параллельный конфликт":
					createErr, wantErr = model.ErrIdempotencyConflict, model.ErrIdempotencyConflict
				case "параллельный повтор":
					stored.StorageKey = ptr("voice/other")
				case "commit подтверждён", "commit не выяснен", "commit другая отправка":
					createErr = model.ErrMediaCommitUnknown
				}
				f.repo.EXPECT().CreateMediaMessage(gomock.Any(), int64(1), model.ChatCreateMedia{ChatID: 10, ClientMessageID: "key", Type: command.Type, StorageKey: "voice/new", DurationSeconds: 2.5, SHA256: mediaDigest(command.Data)}).Return(stored, created, createErr)
				if errors.Is(createErr, model.ErrMediaCommitUnknown) {
					saved, lookupErr := &model.StoredMediaMessage{Message: *stored, SHA256: mediaDigest(command.Data)}, error(nil)
					if name == "commit не выяснен" {
						saved, lookupErr, wantErr = nil, model.ErrMessageNotFound, model.ErrMediaCommitUnknown
					}
					if name == "commit другая отправка" {
						saved.Message.StorageKey = ptr("voice/other")
						stored = &saved.Message
					}
					f.repo.EXPECT().GetMediaByClientID(gomock.Any(), int64(1), int64(10), "key").Return(saved, lookupErr)
				}
				if name == "откат SQL" || name == "ошибка удаления" || name == "параллельный повтор" || name == "параллельный конфликт" || name == "commit другая отправка" {
					deleteErr := error(nil)
					if name == "ошибка удаления" {
						deleteErr = dependencyErr
					}
					f.storage.EXPECT().Delete(gomock.Any(), "voice/new").Return(deleteErr)
				}
				if wantErr == nil {
					f.links.EXPECT().BuildMessageMediaURL(int64(10), int64(100)).Return("/media")
					if created {
						f.repo.EXPECT().GeyMembers(gomock.Any(), int64(10)).Return([]model.ChatMember{}, nil).Times(3)
					}
				}
			}
			got, actualCreated, err := f.svc.SendMedia(context.Background(), 1, command)
			assertError(t, err, wantErr)
			if err == nil && (got.MessageID != 100 || got.Type != command.Type || got.Attachment.URL != "/media" || got.Attachment.DurationSeconds != 2.5 || actualCreated != created) {
				t.Fatalf("message = %v, created = %v", got, actualCreated)
			}
			if err != nil && (got != nil || actualCreated) {
				t.Fatalf("message = %v, created = %v", got, actualCreated)
			}
		})
	}
}

func TestSendMediaVideoDurationLimit(t *testing.T) {
	f := newFixture(t)
	command := mediaCommand()
	command.Type = model.ChatMessageTypeVideoNote
	expectMediaAccess(f)
	f.repo.EXPECT().GetMediaByClientID(gomock.Any(), int64(1), int64(10), "key").Return(nil, model.ErrMessageNotFound)
	f.inspector.EXPECT().Inspect(gomock.Any(), command.Data, command.Type).Return(&model.MediaInfo{ContentType: "video/webm", DurationSeconds: 61}, nil)
	message, created, err := f.svc.SendMedia(context.Background(), 1, command)
	assertError(t, err, model.ErrMediaTooLong)
	if !reflect.DeepEqual(message, (*model.ChatMessage)(nil)) || created {
		t.Fatal("unexpected message")
	}
}
