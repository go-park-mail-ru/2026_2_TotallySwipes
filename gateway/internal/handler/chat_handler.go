package handler

import (
	"context"
	"dating-app/gateway/internal/handler/dto"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
	"strconv"
	"time"

	"dating-app/gateway/internal/model"
)

// ChatHTTPService описывает операции адаптера для HTTP ручек чата.
type ChaHTTPService interface {
	GetSummary(ctx context.Context, userID int64) (*model.ChatSummary, error)
	GetChats(ctx context.Context, userID int64, query model.ChatListQuery) (*model.ChatPage, error)
	GetChat(ctx context.Context, userID, chatID int64) (*model.ChatDetails, error)
	MarkSeen(ctx context.Context, userID, chatID int64) error
	GetMessages(ctx context.Context, userID, chatID int64, query model.ChatMessagesQuery) (*model.ChatMessagesPage, error)

	GetMedia(ctx context.Context, userID, chatID, messageID int64) (string, error)
	SendMedia(ctx context.Context, userID int64, command model.ChatSendMedia) (*model.ChatMessage, bool, error)
}

type ChatHTTPHandler struct {
	svc         ChaHTTPService
	mediaClient *http.Client
}

func NewChatHandler(svc ChaHTTPService) *ChatHTTPHandler {
	return &ChatHTTPHandler{
		svc: svc,
		mediaClient: &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// Summary возвращает абсолютные счётчики текущего пользователя.
func (h *ChatHTTPHandler) Summary(w http.ResponseWriter, r *http.Request, userID int64) {

	summary, err := h.svc.GetSummary(r.Context(), userID)

	if err != nil {
		writeChatServiceError(r.Context(), w, "get chat summary", err, false)
		return
	}
	if summary == nil {
		writeChatServiceError(r.Context(), w, "get chat summary", fmt.Errorf("missing chat summary"), false)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, dto.NewChatSummaryResponse(*summary))
}

// List возвращает страницу чатов текущего пользователя.
func (h *ChatHTTPHandler) List(w http.ResponseWriter, r *http.Request, userID int64) {

	query, errs := dto.ParseChatListQuery(r.URL.Query())

	if len(errs) > 0 {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Некорректные параметры списка чатов", errs)
		return
	}

	page, err := h.svc.GetChats(r.Context(), userID, query)

	if err != nil {
		writeChatServiceError(r.Context(), w, "get chats", err, false)
		return
	}

	if page == nil {
		writeChatServiceError(r.Context(), w, "get chats", fmt.Errorf("missing chat page"), false)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, dto.NewChatPageResponse(*page))
}

// Get возвращает детали чата без изменения позиции прочтения и признака новизны.
func (h *ChatHTTPHandler) Get(w http.ResponseWriter, r *http.Request, userID int64) {

	chatID, ok := readChatPathID(w, r, "chat_id")
	if !ok {
		return
	}

	details, err := h.svc.GetChat(r.Context(), userID, chatID)

	if err != nil {
		writeChatServiceError(r.Context(), w, "get chat", err, false)
		return
	}
	if details == nil {
		writeChatServiceError(r.Context(), w, "get chat", fmt.Errorf("missing chat details"), false)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, dto.NewChatDetailsResponse(*details))
}

// MarkSeen снимает признак нового чата только для текущего пользователя.
func (h *ChatHTTPHandler) MarkSeen(w http.ResponseWriter, r *http.Request, userID int64) {

	chatID, ok := readChatPathID(w, r, "chat_id")
	if !ok {
		return
	}

	if err := h.svc.MarkSeen(r.Context(), userID, chatID); err != nil {
		writeChatServiceError(r.Context(), w, "mark chat seen", err, false)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ChatHTTPHandler) GetMessages(w http.ResponseWriter, r *http.Request, userID int64) {

	chatID, ok := readChatPathID(w, r, "chat_id")
	if !ok {
		return
	}

	query, errs := dto.ParseChatMessagesQuery(r.URL.Query())

	if len(errs) > 0 {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Некорректные параметры истории сообщений", errs)
		return
	}

	page, err := h.svc.GetMessages(r.Context(), userID, chatID, query)

	if err != nil {
		writeChatServiceError(r.Context(), w, "get chat messages", err, false)
		return
	}
	if page == nil {
		writeChatServiceError(r.Context(), w, "get chat messages", fmt.Errorf("missing chat messages page"), false)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, dto.NewChatMessagesPageResponse(*page))
}

func readChatPathID(w http.ResponseWriter, r *http.Request, field string) (int64, bool) {

	id, err := strconv.ParseInt(mux.Vars(r)[field], 10, 64)

	if err != nil || id <= 0 {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Некорректный "+field, nil)
		return 0, false
	}

	return id, true
}
