package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// envReader читает переменные и копит ошибки
type envReader struct {
	errs []error
}

func (r *envReader) err() error {
	return errors.Join(r.errs...)
}

func (r *envReader) fail(key, v string, err error) {
	r.errs = append(r.errs, fmt.Errorf("invalid %s %q: %w", key, v, err))
}

func (r *envReader) required(key string) string {
	v := os.Getenv(key)
	if v == "" {
		r.errs = append(r.errs, fmt.Errorf("missing required environment variable %s", key))
	}
	return v
}

func (r *envReader) str(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (r *envReader) list(key, def string) []string {
	var items []string
	for _, item := range strings.Split(r.str(key, def), ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func (r *envReader) boolean(key string, def bool) bool {
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

// duration допускает 0
func (r *envReader) duration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		r.fail(key, v, err)
		return 0
	}
	if d < 0 {
		r.fail(key, v, errors.New("must not be negative"))
	}
	return d
}

func (r *envReader) positiveDuration(key string, def time.Duration) time.Duration {
	before := len(r.errs)
	d := r.duration(key, def)
	if len(r.errs) == before && d == 0 {
		r.fail(key, os.Getenv(key), errors.New("must be positive"))
	}
	return d
}
