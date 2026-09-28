package service

import "errors"

var (
	ErrActiveTestNotFound = errors.New("active test not found")
	ErrInvalidFeedRequest = errors.New("invalid feed request")
	ErrInvalidTestRequest = errors.New("invalid test request")
	ErrInvalidAnswers     = errors.New("invalid test answers")
	ErrTestNotFound       = errors.New("test not found")
	ErrProfileRequired    = errors.New("profile required")
)
