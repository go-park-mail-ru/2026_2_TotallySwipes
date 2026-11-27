// Package envreader читает конфигурацию микросервисов из переменных окружения,
// накапливая ошибки, чтобы сообщить обо всех неверных переменных сразу.
package envreader

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

type Reader struct {
	errs []error
}

// Err объединяет все ошибки чтения; nil, если их не было.
func (r *Reader) Err() error {
	return errors.Join(r.errs...)
}

// Fail добавляет ошибку проверки, найденную вызывающим кодом.
func (r *Reader) Fail(err error) {
	r.errs = append(r.errs, err)
}

func (r *Reader) fail(key, v string, err error) {
	r.errs = append(r.errs, fmt.Errorf("invalid %s %q: %w", key, v, err))
}

func (r *Reader) Required(key string) string {
	v := os.Getenv(key)
	if v == "" {
		r.errs = append(r.errs, fmt.Errorf("missing required environment variable %s", key))
	}
	return v
}

func (r *Reader) Str(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (r *Reader) Bool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		r.fail(key, v, err)
	}
	return b
}

// PositiveDuration читает длительность больше нуля.
func (r *Reader) PositiveDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		r.fail(key, v, err)
		return def
	}
	if d <= 0 {
		r.fail(key, v, errors.New("must be positive"))
	}
	return d
}

// Port читает номер TCP-порта 1..65535 и возвращает его строкой.
func (r *Reader) Port(key, def string) string {
	v := r.Str(key, def)
	port, err := strconv.Atoi(v)
	if err != nil || port < 1 || port > 65535 {
		r.fail(key, v, errors.New("must be an integer between 1 and 65535"))
	}
	return v
}

func (r *Reader) LogLevel(key string, def slog.Level) slog.Level {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(v)); err != nil {
		r.fail(key, v, err)
		return def
	}
	return level
}
