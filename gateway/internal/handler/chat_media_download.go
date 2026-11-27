package handler

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// GetMedia проверяет доступ через Chat Service и потоково отдаёт объект MinIO.
func (h *ChatHTTPHandler) GetMedia(w http.ResponseWriter, r *http.Request, userID int64) {

	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	chatID, ok := readChatPathID(w, r, "chat_id")
	if !ok {
		return
	}
	messageID, ok := readChatPathID(w, r, "message_id")
	if !ok {
		return
	}

	rangeValue := r.Header.Get("Range")

	if rangeValue != "" && !validSingleByteRange(rangeValue) {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Поддерживается один диапазон Range в байтах", nil)
		return
	}

	downloadURL, err := h.svc.GetMedia(r.Context(), userID, chatID, messageID)

	if err != nil {
		writeChatServiceError(r.Context(), w, "get chat media URL", err, true)
		return
	}

	target, err := url.Parse(downloadURL)

	if err != nil || target.Host == "" || target.User != nil || target.Scheme != "http" && target.Scheme != "https" {
		writeChatServiceError(r.Context(), w, "get chat media URL", fmt.Errorf("proto: invalid download_url"), true)
		return
	}

	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target.String(), nil)

	if err != nil {
		writeChatServiceError(r.Context(), w, "create media request", err, true)
		return
	}

	request.Header.Set("Accept-Encoding", "identity")
	if rangeValue != "" {
		request.Header.Set("Range", rangeValue)
	}

	response, err := h.mediaClient.Do(request)

	if err != nil {
		writeChatServiceError(r.Context(), w, "download chat media", err, true)
		return
	}
	defer response.Body.Close()

	switch response.StatusCode {

	case http.StatusOK, http.StatusPartialContent:
		for _, header := range []string{"Content-Type", "Content-Length", "Content-Range"} {
			if value := response.Header.Get(header); value != "" {
				w.Header().Set(header, value)
			}
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(response.StatusCode)

		if _, err := io.Copy(w, response.Body); err != nil {
			slog.ErrorContext(r.Context(), "copy chat media response", "error", err)
		}

	case http.StatusRequestedRangeNotSatisfiable:
		w.Header().Set("Accept-Ranges", "bytes")
		if value := response.Header.Get("Content-Range"); value != "" {
			w.Header().Set("Content-Range", value)
		}
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)

	case http.StatusNotFound:
		writeError(r.Context(), w, http.StatusNotFound, "MEDIA_NOT_FOUND", "Медиафайл не найден", nil)

	default:
		writeChatServiceError(r.Context(), w, "download chat media", fmt.Errorf("storage returned status %d", response.StatusCode), true)
	}
}

func validSingleByteRange(value string) bool {

	if !strings.HasPrefix(value, "bytes=") {
		return false
	}

	first, last, ok := strings.Cut(strings.TrimPrefix(value, "bytes="), "-")

	if !ok || first == "" && last == "" {
		return false
	}

	if first == "" {
		suffix, err := strconv.ParseInt(last, 10, 64)
		return err == nil && suffix > 0 && !strings.HasPrefix(last, "+")
	}

	start, err := strconv.ParseInt(first, 10, 64)

	if err != nil || start < 0 || strings.HasPrefix(first, "+") {
		return false
	}

	if last == "" {
		return true
	}

	end, err := strconv.ParseInt(last, 10, 64)
	return err == nil && end >= start && !strings.HasPrefix(last, "+")
}
