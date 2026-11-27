package model

import "errors"

var (
	ErrNotFound               = errors.New("not found")
	ErrInvalidMatchesRequest  = errors.New("invalid matches request")
	ErrInvalidFeedRequest     = errors.New("invalid feed request")
	ErrInvalidSwipe           = errors.New("invalid swipe")
	ErrProfileRequired        = errors.New("profile required")
	ErrProfileIncomplete      = errors.New("profile incomplete")
	ErrProfileNotFound        = errors.New("profile not found")
	ErrLocationRequired       = errors.New("location required")
	ErrMatchNotFound          = errors.New("match not found")
	ErrInvalidBigFive         = errors.New("invalid Big Five")
	ErrInvalidEventResults    = errors.New("invalid event results")
	ErrNoEvents               = errors.New("no events")
	ErrTimepadTimeout         = errors.New("timepad timeout")
	ErrTimepadStatusCode      = errors.New("timepad status code")
	ErrResultAlreadySubmitted = errors.New("result already submitted")
	ErrSearchStateConflict    = errors.New("search state conflict")
)
