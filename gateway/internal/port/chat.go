package port

import "dating-app/gateway/internal/model"

type ChatEventStream interface {
	Recv() (*model.ChatEvent, error)
}
