package handler

import (
	"context"
	"dating-app/gateway/internal/config"
	"dating-app/gateway/internal/handler/dto"
	"dating-app/gateway/internal/middleware"
	"dating-app/gateway/internal/model"
	"dating-app/gateway/internal/port"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type ChatService interface {
	SubscribeEvents(ctx context.Context, userID int64) (port.ChatEventStream, error)
	SendMessage(ctx context.Context, userID int64, command model.ChatSendMessage) (*model.ChatMessage, error)
	EditMessage(ctx context.Context, userID int64, command model.ChatEditMessage) (*model.ChatMessage, error)
	DeleteMessage(ctx context.Context, userID int64, command model.ChatDeleteMessage) (*model.ChatMessage, error)
	ReadChat(ctx context.Context, userID int64, command model.ChatRead) (*model.ChatReadUpdated, error)
}

type WSHandler struct {
	upgrader websocket.Upgrader
	cfg      config.WebSocketConfig
	chat     ChatService

	mu           sync.Mutex
	connections  map[*websocket.Conn]context.CancelFunc
	closing      bool
	wg           sync.WaitGroup
	shutdownOnce sync.Once
	done         chan struct{}
}

func NewWSHandler(cfg config.WebSocketConfig, chat ChatService) *WSHandler {

	allowedOrigins := make(map[string]bool, len(cfg.AllowedOrigins))
	for _, origin := range cfg.AllowedOrigins {
		allowedOrigins[origin] = true
	}

	return &WSHandler{
		cfg:         cfg,
		chat:        chat,
		connections: make(map[*websocket.Conn]context.CancelFunc),
		done:        make(chan struct{}),
		upgrader: websocket.Upgrader{
			HandshakeTimeout: cfg.HandshakeTimeout,
			CheckOrigin: func(r *http.Request) bool {
				return allowedOrigins[r.Header.Get("Origin")]
			},
		},
	}
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())

	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !h.beginConnection() {
		http.Error(w, "Server is shutting down", http.StatusServiceUnavailable)
		return
	}

	defer h.wg.Done()

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	if !h.registerConnection(conn, cancel) {
		return
	}

	defer h.unregisterConnection(conn)

	h.serveConnection(ctx, conn, userID)
}

type wsConnection struct {
	conn     *websocket.Conn
	userID   int64
	send     chan []byte
	commands chan []byte
}

func (h *WSHandler) serveConnection(ctx context.Context, conn *websocket.Conn, userID int64) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	connection := &wsConnection{
		conn:     conn,
		userID:   userID,
		send:     make(chan []byte, h.cfg.SendQueueSize),
		commands: make(chan []byte, h.cfg.CommandsQueueSize),
	}

	conn.SetReadLimit(h.cfg.MaxMessageBytes)

	if err := conn.SetReadDeadline(time.Now().Add(h.cfg.PongTimeout)); err != nil {
		conn.Close()
		return
	}

	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(h.cfg.PongTimeout))
	})

	var wg sync.WaitGroup

	startLoop := func(name string, loop func(context.Context, *wsConnection) error) {
		wg.Add(1)

		go func() {
			defer wg.Done()
			defer cancel()
			defer conn.Close()

			if err := loop(ctx, connection); err != nil && ctx.Err() == nil {

				slog.DebugContext(
					ctx,
					"websocket loop stopped",
					"loop", name,
					"user_id", userID,
					"error", err,
				)
			}
		}()
	}

	startLoop("read", h.readLoop)
	startLoop("write", h.writeLoop)
	startLoop("events", h.eventLoop)
	startLoop("commands", h.commandLoop)

	<-ctx.Done()
	conn.Close()

	wg.Wait()
}

func (h *WSHandler) eventLoop(ctx context.Context, connection *wsConnection) error {

	stream, err := h.chat.SubscribeEvents(ctx, connection.userID)

	if err != nil {
		return err
	}

	for {
		event, err := stream.Recv()

		if err != nil {
			return err
		}

		response, err := dto.NewChatEvent(event)

		if err != nil {
			return err
		}

		payload, err := json.Marshal(response)

		if err != nil {
			return err
		}

		if err := connection.enqueue(ctx, payload); err != nil {
			return err
		}
	}
}

func (c *wsConnection) enqueue(ctx context.Context, payload []byte) error {

	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.send <- payload:
		return nil
	default:
		return fmt.Errorf("websocket: send queue is full")
	}
}

func (h *WSHandler) writeLoop(ctx context.Context, connection *wsConnection) error {

	tiсker := time.NewTicker(h.cfg.PingInterval)
	defer tiсker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case payload := <-connection.send:

			if err := connection.conn.SetWriteDeadline(time.Now().Add(h.cfg.WriteTimeout)); err != nil {
				return err
			}

			if err := connection.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return err
			}
		case <-tiсker.C:
			if err := connection.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(h.cfg.WriteTimeout)); err != nil {
				return err
			}
		}
	}
}

func (h *WSHandler) readLoop(ctx context.Context, connection *wsConnection) error {

	for {
		messageType, payload, err := connection.conn.ReadMessage()

		if err != nil {
			return err
		}

		if messageType != websocket.TextMessage {
			return fmt.Errorf("websocket: expected text message")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case connection.commands <- payload:
		default:
			return fmt.Errorf("websocket: command queue is full")
		}
	}
}

func (h *WSHandler) commandLoop(ctx context.Context, connection *wsConnection) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case payload := <-connection.commands:

			if err := h.handleCommand(ctx, connection, payload); err != nil {
				return err
			}
		}
	}
}

func (h *WSHandler) handleCommand(ctx context.Context, connection *wsConnection, payload []byte) error {
	var command dto.ChatCommand

	if err := json.Unmarshal(payload, &command); err != nil {
		return connection.sendCommandError(ctx, nil, "VALIDATION_ERROR", "Некорректный JSON команды")
	}

	if strings.TrimSpace(command.RequestID) == "" {
		return connection.sendCommandError(ctx, nil, "VALIDATION_ERROR", "Необходимо передать request_id")
	}

	data, err := dto.ParseChatCommandData(command)

	if err != nil {
		return connection.sendCommandError(ctx, &command.RequestID, "VALIDATION_ERROR", err.Error())
	}

	result, err := h.executeCommand(ctx, connection.userID, data)

	if err != nil {
		return connection.sendCommandError(ctx, &command.RequestID, "INTERNAL_SERVER_ERROR", "Не удалось выполнить команду")
	}

	response := dto.NewChatCommandResult(command.RequestID, result)

	return connection.sendJSON(ctx, response)

}

func (h *WSHandler) executeCommand(ctx context.Context, userID int64, command any) (any, error) {

	switch data := command.(type) {
	case model.ChatSendMessage:
		message, err := h.chat.SendMessage(ctx, userID, data)

		if err != nil {
			return nil, err
		}

		return dto.MessageCreatedData{
			ChatID:          strconv.FormatInt(data.ChatID, 10),
			ClientMessageID: &data.ClientMessageID,
			Message:         dto.NewChatMessageResponse(*message),
		}, nil

	case model.ChatEditMessage:
		message, err := h.chat.EditMessage(ctx, userID, data)

		if err != nil {
			return nil, err
		}

		return dto.MessageChangedData{
			ChatID:  strconv.FormatInt(data.ChatID, 10),
			Message: dto.NewChatMessageResponse(*message),
		}, nil

	case model.ChatDeleteMessage:
		message, err := h.chat.DeleteMessage(ctx, userID, data)

		if err != nil {
			return nil, err
		}

		return dto.MessageChangedData{
			ChatID:  strconv.FormatInt(data.ChatID, 10),
			Message: dto.NewChatMessageResponse(*message),
		}, nil

	case model.ChatRead:
		read, err := h.chat.ReadChat(ctx, userID, data)

		if err != nil {
			return nil, err
		}

		return dto.ChatReadUpdated{
			ChatID:      strconv.FormatInt(read.ChatID, 10),
			UserID:      read.UserID,
			LastReadSeq: read.LastReadSeq,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported chat command: %T", command)
	}
}

func (c *wsConnection) sendJSON(ctx context.Context, response any) error {

	payload, err := json.Marshal(response)

	if err != nil {
		return err
	}

	return c.enqueue(ctx, payload)
}

func (c *wsConnection) sendCommandError(ctx context.Context, requestID *string, code string, message string) error {

	response := dto.NewChatCommandError(requestID, code, message)

	return c.sendJSON(ctx, response)
}
