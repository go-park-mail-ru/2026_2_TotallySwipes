package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"dating-app/gateway/internal/handler/dto"
	"dating-app/gateway/internal/model"

	"github.com/gorilla/mux"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	maxVoiceUploadSize            = 5 << 20
	maxVideoNoteUploadSize        = 10 << 20
	mediaUploadMaxBody            = maxVideoNoteUploadSize + (1 << 20)
	mediaUploadMaxMemory          = 1 << 20
	maxMediaClientMessageIDLength = 128
)

// SendMedia принимает один файл и сразу создаёт сообщение через Chat Service.
func (h *ChatHTTPHandler) SendMedia(w http.ResponseWriter, r *http.Request, userID int64) {

	chatID, err := strconv.ParseInt(mux.Vars(r)["chat_id"], 10, 64)

	if err != nil || chatID <= 0 {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Некорректный chat_id", nil)
		return
	}

	command, ok := readChatMediaUpload(w, r)

	if !ok {
		return
	}

	command.ChatID = chatID

	message, created, err := h.svc.SendMedia(r.Context(), userID, command)

	if err != nil {
		writeChatMediaError(r.Context(), w, err)
		return
	}

	if message == nil {
		slog.ErrorContext(r.Context(), "send chat media returned no message", "chat_id", chatID)
		writeError(r.Context(), w, http.StatusInternalServerError, "MEDIA_SEND_FAILED", "Не удалось отправить голосовое или видео. Попробуйте ещё раз.", nil)
		return
	}

	responseStatus := http.StatusOK

	if created {
		responseStatus = http.StatusCreated
	}

	writeJSON(r.Context(), w, responseStatus, dto.NewChatMessageResponse(*message))
}

func readChatMediaUpload(w http.ResponseWriter, r *http.Request) (model.ChatSendMedia, bool) {

	var command model.ChatSendMedia

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))

	if err != nil || mediaType != "multipart/form-data" {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Ожидается multipart/form-data", nil)
		return command, false
	}

	r.Body = http.MaxBytesReader(w, r.Body, mediaUploadMaxBody)
	err = r.ParseMultipartForm(mediaUploadMaxMemory)

	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	if err != nil {

		var maxBytesErr *http.MaxBytesError

		if errors.As(err, &maxBytesErr) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, codePayloadTooLarge, "Слишком большой запрос", nil)
		} else {
			writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Некорректный multipart-запрос", nil)
		}

		return command, false
	}

	form := r.MultipartForm

	if len(form.File) != 1 || len(form.File["file"]) != 1 ||
		len(form.Value) != 2 || len(form.Value["type"]) != 1 || len(form.Value["client_message_id"]) != 1 {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Нужны ровно один file, type и client_message_id без дополнительных полей", nil)
		return command, false
	}

	command.ClientMessageID = form.Value["client_message_id"][0]

	if !utf8.ValidString(command.ClientMessageID) ||
		strings.TrimSpace(command.ClientMessageID) == "" ||
		utf8.RuneCountInString(command.ClientMessageID) > maxMediaClientMessageIDLength {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Некорректный client_message_id", nil)
		return command, false
	}

	command.Type = model.ChatMessageType(form.Value["type"][0])
	maxSize := int64(maxVideoNoteUploadSize)

	switch command.Type {

	case model.ChatMessageTypeVoice:
		maxSize = maxVoiceUploadSize

	case model.ChatMessageTypeVideoNote:

	default:
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Допустимые type: voice, video_note", nil)
		return command, false
	}

	fileHeader := form.File["file"][0]

	if fileHeader.Size > maxSize {
		writeError(r.Context(), w, http.StatusRequestEntityTooLarge, codePayloadTooLarge, "Слишком большой файл", nil)
		return command, false
	}

	file, err := fileHeader.Open()

	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Не удалось прочитать файл", nil)
		return command, false
	}
	defer file.Close()

	command.Data, err = io.ReadAll(io.LimitReader(file, maxSize+1))

	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Не удалось прочитать файл", nil)
		return model.ChatSendMedia{}, false
	}

	if int64(len(command.Data)) > maxSize {
		writeError(r.Context(), w, http.StatusRequestEntityTooLarge, codePayloadTooLarge, "Слишком большой файл", nil)
		return model.ChatSendMedia{}, false
	}

	if len(command.Data) == 0 {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Файл пуст", nil)
		return command, false
	}

	return command, true
}

func writeChatMediaError(ctx context.Context, w http.ResponseWriter, err error) {

	switch status.Code(err) {

	case codes.InvalidArgument:

		for _, detail := range status.Convert(err).Details() {

			if info, ok := detail.(*errdetails.ErrorInfo); ok && info.GetReason() == "UNSUPPORTED_MEDIA_TYPE" && info.GetDomain() == "chat" {
				writeError(ctx, w, http.StatusUnsupportedMediaType, codeUnsupportedMediaType, "Формат медиа не поддерживается", nil)
				return
			}
		}
		writeError(ctx, w, http.StatusBadRequest, codeValidationError, "Некорректные данные медиа", nil)

	case codes.Unauthenticated:
		writeError(ctx, w, http.StatusUnauthorized, codeUnauthorized, msgUnauthorized, nil)

	case codes.PermissionDenied:
		writeError(ctx, w, http.StatusForbidden, "MESSAGING_NOT_ALLOWED", "Отправка сообщений недоступна", nil)

	case codes.NotFound:
		writeError(ctx, w, http.StatusNotFound, "CHAT_NOT_FOUND", "Чат не найден или недоступен", nil)

	case codes.AlreadyExists:
		writeError(ctx, w, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "ID сообщения уже использован", nil)

	case codes.ResourceExhausted:
		writeError(ctx, w, http.StatusRequestEntityTooLarge, codePayloadTooLarge, "Превышен допустимый размер медиа или лимит ресурсов", nil)

	default:
		slog.ErrorContext(ctx, "send chat media failed", "error", err)
		writeError(ctx, w, http.StatusInternalServerError, "MEDIA_SEND_FAILED", "Не удалось отправить голосовое или видео. Попробуйте ещё раз.", nil)
	}
}
