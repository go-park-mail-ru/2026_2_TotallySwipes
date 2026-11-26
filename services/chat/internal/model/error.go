package model

import "errors"

var (
	ErrInvalidUserID     = errors.New("invalid user id")
	ErrChatMemberBlocked = errors.New("chat member blocked")
)
