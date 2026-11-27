package grpcerr_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"dating-app/internal/grpcerr"
	"dating-app/internal/model"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Все ошибки, которые различает gateway/internal/handler/errors.go, должны переживать gRPC.
var gatewayErrors = []error{
	model.ErrInvalidEventResults, model.ErrNoEvents, model.ErrResultAlreadySubmitted, model.ErrSearchStateConflict,
	model.ErrTimepadTimeout, model.ErrInvalidCoordinates, model.ErrLocationRequired, model.ErrLocationPreviewUnavailable,
	model.ErrLocationNotFound, model.ErrCityNotFound, model.ErrGeocoderTimeout, model.ErrGeocoderUnavailable,
	model.ErrGeocoderQuotaExceeded, model.ErrInvalidCredentials, model.ErrInvalidSession, model.ErrEmailAlreadyExists,
	model.ErrPasswordTooLong, model.ErrUnknownTag, model.ErrInvalidFeedRequest, model.ErrInvalidMatchesRequest,
	model.ErrInvalidTestRequest, model.ErrInvalidAnswers, model.ErrProfileRequired, model.ErrProfileIncomplete,
	model.ErrPhotoLimit, model.ErrLastPhoto, model.ErrMatchNotFound, model.ErrPhotoNotFound, model.ErrPhotoOrderMismatch,
	model.ErrTestNotFound, model.ErrTestResultNotFound, model.ErrInvalidSwipe, model.ErrProfileNotFound,
	model.ErrNotFound, model.ErrSessionNotOpened, model.ErrInvalidPhotoStorageKey,
}

func TestRoundTripKeepsDomainError(t *testing.T) {
	for _, domainErr := range gatewayErrors {
		t.Run(domainErr.Error(), func(t *testing.T) {
			wrapped := fmt.Errorf("op user id=1: %w", domainErr)

			st := grpcerr.ToStatus(wrapped)
			if code := status.Code(st); code == codes.Internal && !errors.Is(domainErr, model.ErrInvalidBigFive) {
				// Internal допустим только для ошибок, которые и в монолите были 500
				t.Fatalf("code = Internal for domain error %v", domainErr)
			}

			got := grpcerr.FromStatus(st)
			if !errors.Is(got, domainErr) {
				t.Fatalf("FromStatus(ToStatus(%v)) = %v, want errors.Is", domainErr, got)
			}
			if _, ok := status.FromError(got); !ok {
				t.Fatalf("restored error lost gRPC status: %v", got)
			}
		})
	}
}

func TestMoreSpecificErrorWins(t *testing.T) {
	// Двойное оборачивание, как в AuthService.Register: сессия не открылась из-за ErrNotFound.
	err := fmt.Errorf("register: %w: %w", model.ErrSessionNotOpened, model.ErrNotFound)

	got := grpcerr.FromStatus(grpcerr.ToStatus(err))
	if !errors.Is(got, model.ErrSessionNotOpened) {
		t.Fatalf("got %v, want ErrSessionNotOpened", got)
	}
}

func TestMetadataTravelsWithError(t *testing.T) {
	err := grpcerr.WithMetadata(model.ErrSessionNotOpened, map[string]string{grpcerr.MetadataUserID: "42"})

	got := grpcerr.FromStatus(grpcerr.ToStatus(err))
	if v := grpcerr.Metadata(got)[grpcerr.MetadataUserID]; v != "42" {
		t.Fatalf("metadata user_id = %q, want 42", v)
	}
}

func TestUnknownErrorHidesDetails(t *testing.T) {
	st := grpcerr.ToStatus(errors.New("pq: password authentication failed for user auth_app"))

	if status.Code(st) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(st))
	}
	if msg := status.Convert(st).Message(); msg != "internal error" {
		t.Fatalf("message leaked: %q", msg)
	}
	if got := grpcerr.FromStatus(st); status.Code(got) != codes.Internal {
		t.Fatalf("FromStatus changed unknown status: %v", got)
	}
}

func TestContextAndStatusErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want codes.Code
	}{
		{"canceled", fmt.Errorf("query: %w", context.Canceled), codes.Canceled},
		{"deadline", fmt.Errorf("query: %w", context.DeadlineExceeded), codes.DeadlineExceeded},
		{"status passthrough", status.Error(codes.InvalidArgument, "bad mask"), codes.InvalidArgument},
		{"nil", nil, codes.OK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := status.Code(grpcerr.ToStatus(tt.err)); got != tt.want {
				t.Fatalf("code = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFromStatusIgnoresPlainErrors(t *testing.T) {
	plain := errors.New("dial failed")
	if got := grpcerr.FromStatus(plain); got != plain {
		t.Fatalf("FromStatus(plain) = %v", got)
	}
	if grpcerr.FromStatus(nil) != nil {
		t.Fatal("FromStatus(nil) != nil")
	}
}
