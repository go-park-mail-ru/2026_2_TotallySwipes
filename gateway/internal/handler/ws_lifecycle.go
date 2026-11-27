package handler

import (
	"context"
	"time"

	"github.com/gorilla/websocket"
)

func (h *WSHandler) beginConnection() bool {

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closing {
		return false
	}

	h.wg.Add(1)

	return true
}

func (h *WSHandler) registerConnection(conn *websocket.Conn, cancel context.CancelFunc) bool {

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closing {
		return false
	}

	h.connections[conn] = cancel

	return true
}

func (h *WSHandler) unregisterConnection(conn *websocket.Conn) {

	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.connections, conn)
}

// Shutdown закрывает сокеты, отменяет подписки и ожидает завершения всех циклов.
func (h *WSHandler) Shutdown(ctx context.Context) error {

	h.shutdownOnce.Do(func() {

		h.mu.Lock()
		h.closing = true
		connections := make(map[*websocket.Conn]context.CancelFunc, len(h.connections))

		for conn, cancel := range h.connections {
			connections[conn] = cancel
		}

		h.mu.Unlock()

		for conn, cancel := range connections {
			go func() {

				deadline := time.Now().Add(h.cfg.WriteTimeout)

				if shutdownDeadline, ok := ctx.Deadline(); ok && shutdownDeadline.Before(deadline) {
					deadline = shutdownDeadline
				}

				_ = conn.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down"),
					deadline,
				)

				cancel()
				_ = conn.Close()
			}()
		}

		go func() {
			h.wg.Wait()
			close(h.done)
		}()
	})

	select {
	case <-h.done:
		return nil

	case <-ctx.Done():

		h.mu.Lock()

		for conn, cancel := range h.connections {
			cancel()
			_ = conn.Close()
		}

		h.mu.Unlock()

		return ctx.Err()
	}
}
