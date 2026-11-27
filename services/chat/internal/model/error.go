package model

import "errors"

var (
	ErrInvalidMedia       = errors.New("invalid media")
	ErrUnsupportedMedia   = errors.New("unsupported media format")
	ErrMediaTooLarge      = errors.New("media is too large")
	ErrMediaTooLong       = errors.New("media duration exceeds limit")
	ErrMediaCommitUnknown = errors.New("media commit result is unknown")

	ErrInvalidUserID     = errors.New("invalid user id")
	ErrInvalidMatchID    = errors.New("invalid match id")
	ErrChatMemberBlocked = errors.New("chat member blocked")

	ErrInvalidChatID        = errors.New("invalid chat id")
	ErrInvalidMessagesQuery = errors.New("invalid messages query")
	ErrChatNotFound         = errors.New("chat not found")

	ErrIdempotencyConflict    = errors.New("idempotency conflict")
	ErrMessagingNotAllowed    = errors.New("messaging not allowed")
	ErrInvalidClientMessageID = errors.New("invalid client message id")
	ErrInvalidMessageText     = errors.New("invalid message text")
	ErrInvalidReadSeq         = errors.New("invalid read seq")
	ErrInvalidMessageID       = errors.New("invalid message id")
	ErrMessageNotFound        = errors.New("message not found")
	ErrMediaNotFound          = errors.New("media not found")
	ErrInvalidChatMembers     = errors.New("invalid chat members")
)
