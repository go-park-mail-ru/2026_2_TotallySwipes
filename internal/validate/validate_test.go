package validate

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		in   string
		want error
	}{
		{"alex@example.com", nil},
		{"a.b+tag@sub.example.ru", nil},
		{"", ErrRequired},
		{"alex", ErrEmailFormat},
		{"alex@example", ErrEmailFormat},
		{"@example.com", ErrEmailFormat},
		{"al ex@example.com", ErrEmailFormat},
		{"a@b@example.com", ErrEmailFormat},
		{"a@.b.com", ErrEmailFormat},
		{"a@b..com", ErrEmailFormat},
		{"a@b.com.", ErrEmailFormat},
		{"a@-b.com", ErrEmailFormat},
		{"a\x00b@example.com", ErrEmailFormat},
		{"a@b\u00a0c.com", ErrEmailFormat},
		{strings.Repeat("a", 250) + "@x.ru", ErrEmailTooLong},
	}
	for _, tt := range tests {
		if got := ValidateEmail(tt.in); !errors.Is(got, tt.want) {
			t.Errorf("ValidateEmail(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		in   string
		want error
	}{
		{"qwerty123", nil},
		{"пароль123", nil},
		{" spaces 1 ", nil},
		{"", ErrRequired},
		{"abc123", ErrPasswordTooShort},
		{"abcdefgh", ErrPasswordWeak},
		{"12345678", ErrPasswordWeak},
		{strings.Repeat("я", 127) + "1", nil},
		{strings.Repeat("a", 128) + "1", ErrPasswordTooLong},
	}
	for _, tt := range tests {
		if got := ValidatePassword(tt.in); !errors.Is(got, tt.want) {
			t.Errorf("ValidatePassword(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestValidateLoginPassword(t *testing.T) {
	tests := []struct {
		in   string
		want error
	}{
		{"x", nil},
		{"abc", nil},
		{strings.Repeat("я", 128), nil},
		{"", ErrRequired},
		{strings.Repeat("я", 129), ErrPasswordTooLong},
	}
	for _, tt := range tests {
		if got := ValidateLoginPassword(tt.in); !errors.Is(got, tt.want) {
			t.Errorf("ValidateLoginPassword(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		in   string
		want error
	}{
		{"alex", nil},
		{"Анна-Мария", nil},
		{strings.Repeat("я", 100), nil},
		{"", ErrRequired},
		{"   ", ErrNameBlank},
		{strings.Repeat("я", 101), ErrNameTooLong},
	}
	for _, tt := range tests {
		if got := ValidateName(tt.in); !errors.Is(got, tt.want) {
			t.Errorf("ValidateName(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestValidateAgeAt(t *testing.T) {
	now := time.Date(2026, 9, 26, 15, 0, 0, 0, time.UTC)
	date := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

	tests := []struct {
		name  string
		birth time.Time
		want  error
	}{
		{"ровно 18 сегодня", date(2008, 9, 26), nil},
		{"18 исполнится завтра", date(2008, 9, 27), ErrUnderage},
		{"взрослый", date(1990, 1, 1), nil},
		{"дата в будущем", date(2030, 1, 1), ErrUnderage},
		{"29 февраля, 18 уже есть", date(2008, 2, 29), nil},
	}
	for _, tt := range tests {
		if got := validateAgeAt(tt.birth, now); !errors.Is(got, tt.want) {
			t.Errorf("%s: validateAgeAt(%v) = %v, want %v", tt.name, tt.birth, got, tt.want)
		}
	}

	leap := date(2008, 2, 29)
	if err := validateAgeAt(leap, date(2026, 2, 28)); !errors.Is(err, ErrUnderage) {
		t.Errorf("29.02 на 28.02: got %v, want %v", err, ErrUnderage)
	}
	if err := validateAgeAt(leap, date(2026, 3, 1)); err != nil {
		t.Errorf("29.02 на 01.03: got %v, want nil", err)
	}
}

func TestParseBirthDate(t *testing.T) {
	tests := []struct {
		in   string
		want error
	}{
		{"2005-08-17", nil},
		{"", ErrRequired},
		{"17.08.2005", ErrDateFormat},
		{"2005-02-30", ErrDateFormat},
		{"2005-8-17", ErrDateFormat},
	}
	for _, tt := range tests {
		if _, got := ParseBirthDate(tt.in); !errors.Is(got, tt.want) {
			t.Errorf("ParseBirthDate(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestEnums(t *testing.T) {
	tests := []struct {
		name string
		fn   func(string) error
		in   string
		want error
	}{
		{"sex ok", ValidateSex, "female", nil},
		{"sex all", ValidateSex, "all", ErrSex},
		{"sex empty", ValidateSex, "", ErrRequired},
		{"sex case", ValidateSex, "Male", ErrSex},
		{"search_sex all", ValidateSearchSex, "all", nil},
		{"search_sex bad", ValidateSearchSex, "other", ErrSearchSex},
		{"intent ok", ValidateDatingIntent, "Ищу половинку", nil},
		{"intent bad", ValidateDatingIntent, "Ищу работу", ErrDatingIntent},
		{"intent empty", ValidateDatingIntent, "", ErrRequired},
	}
	for _, tt := range tests {
		if got := tt.fn(tt.in); !errors.Is(got, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestValidateTags(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want error
	}{
		{"nil", nil, nil},
		{"ok", []string{"sport", "gaming"}, nil},
		{"много", strings.Split("a,b,c,d,e,f,g,h,i,j,k", ","), ErrTagsTooMany},
		{"дубль", []string{"sport", "sport"}, ErrTagsNotUniq},
		{"пустой", []string{""}, ErrTagInvalid},
		{"пробелы", []string{"  "}, ErrTagInvalid},
		{"длинный", []string{strings.Repeat("я", 31)}, ErrTagInvalid},
	}
	for _, tt := range tests {
		if got := ValidateTags(tt.in); !errors.Is(got, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestValidateSearchAge(t *testing.T) {
	tests := []struct {
		in   int
		want error
	}{
		{18, nil},
		{99, nil},
		{17, ErrSearchAgeTooLow},
		{0, ErrSearchAgeTooLow},
		{100, nil},
		{101, ErrSearchAgeTooHigh},
		{3000000000, ErrSearchAgeTooHigh},
	}
	for _, tt := range tests {
		if got := ValidateSearchAge(tt.in); !errors.Is(got, tt.want) {
			t.Errorf("ValidateSearchAge(%d) = %v, want %v", tt.in, got, tt.want)
		}
	}

	if err := ValidateSearchAgeRange(20, 20); err != nil {
		t.Errorf("ValidateSearchAgeRange(20, 20) = %v, want nil", err)
	}
	if err := ValidateSearchAgeRange(30, 20); !errors.Is(err, ErrSearchAgeRange) {
		t.Errorf("ValidateSearchAgeRange(30, 20) = %v, want %v", err, ErrSearchAgeRange)
	}
}
