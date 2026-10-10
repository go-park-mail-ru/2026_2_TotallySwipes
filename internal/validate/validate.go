package validate

import (
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"dating-app/internal/model"
)

const (
	emailMaxLen       = 254
	emailLocalMaxLen  = 64
	emailDomainMinLen = 4

	passwordMinLen = 8
	passwordMaxLen = 128

	nameMaxLen = 64

	tagsMaxCount = 10
	tagMaxLen    = 30

	aboutMeMaxLen = 1000

	workMaxLen = 100

	PhotoMaxSize = 5 << 20

	minAge = 18

	DateLayout = "2006-01-02"
)

var (
	ErrRequired = errors.New("обязательное поле")

	ErrEmailFormat  = errors.New("некорректный email")
	ErrEmailTooLong = errors.New("email слишком длинный")

	ErrPasswordTooShort = errors.New("пароль должен быть не короче 8 символов")
	ErrPasswordTooLong  = errors.New("пароль слишком длинный")
	ErrPasswordWeak     = errors.New("пароль должен содержать хотя бы одну букву и одну цифру")

	ErrNameTooLong = errors.New("имя должно быть не длиннее 64 символов")
	ErrNameFormat  = errors.New("имя должно начинаться с буквы и может содержать буквы, пробел, дефис, апостроф и точку, без двух разделителей подряд")

	ErrDateFormat = errors.New("дата должна быть в формате YYYY-MM-DD")
	ErrUnderage   = errors.New("пользователю должно быть не меньше 18 лет")

	ErrSex        = errors.New("допустимые значения: male, female")
	ErrSearchSex  = errors.New("допустимые значения: male, female, all")
	ErrDatingGoal = errors.New("допустимые значения: relationship, friendship, casual")
	ErrEducation  = errors.New("допустимые значения: secondary, vocational, incomplete_higher, higher, degree")
	ErrAttitude   = errors.New("допустимые значения: negative, neutral, positive")

	ErrSearchAgeTooLow  = errors.New("возраст для поиска должен быть не меньше 18")
	ErrSearchAgeTooHigh = errors.New("возраст для поиска должен быть не больше 100")
	ErrSearchAgeRange   = errors.New("верхняя граница возраста не может быть меньше нижней")

	ErrTagsTooMany = errors.New("не больше 10 тегов")
	ErrTagsNotUniq = errors.New("теги не должны повторяться")
	ErrTagInvalid  = errors.New("тег должен быть от 1 до 30 символов и не состоять только из пробелов")
	ErrTagUnknown  = errors.New("неизвестный тег")

	ErrNotInteger = errors.New("должно быть целым числом")

	ErrAboutMeTooLong = errors.New("описание должно быть не длиннее 1000 символов")
	ErrWorkTooLong    = errors.New("работа должна быть не длиннее 100 символов")
	ErrHeight         = errors.New("рост должен быть от 100 до 250 см")
	ErrPhotoTooLarge  = errors.New("фото должно быть не больше 5 МБ")
	ErrPhotoFormat    = errors.New("фото должно быть в формате JPEG, PNG или WebP")
	ErrPhotoOrder     = errors.New("нужно передать id всех фото анкеты, каждый один раз")
)

// emailRe совпадает с регуляркой фронта: буквы любого алфавита в локальной части и домене
var emailRe = regexp.MustCompile(`^[\p{L}0-9][\p{L}\p{M}0-9'_+&*-]*(?:\.[\p{L}\p{M}0-9'_+&*-]+)*` +
	`@(?:[\p{L}0-9](?:[\p{L}\p{M}0-9-]{0,61}[\p{L}\p{M}0-9])?\.)+` +
	`(?:\p{L}[\p{L}\p{M}]{1,62}|xn--[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,57}[a-zA-Z0-9])?)$`)

var photoExtByContentType = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

var (
	sexValues       = []string{"male", "female"}
	searchSexValues = []string{"male", "female", "all"}
)

// ValidateEmail ожидает уже обрезанную по краям строку в lower case
func ValidateEmail(s string) error {
	if s == "" {
		return ErrRequired
	}
	if utf8.RuneCountInString(s) > emailMaxLen {
		return ErrEmailTooLong
	}
	at := strings.LastIndexByte(s, '@')
	if at < 0 {
		return ErrEmailFormat
	}
	local, domain := s[:at], s[at+1:]
	if local == "" || utf8.RuneCountInString(local) > emailLocalMaxLen {
		return ErrEmailFormat
	}
	if utf8.RuneCountInString(domain) < emailDomainMinLen {
		return ErrEmailFormat
	}
	if !emailRe.MatchString(s) || mixedScriptLabel(domain) {
		return ErrEmailFormat
	}
	return nil
}

// mixedScriptLabel - есть ли метка домена с латиницей и кириллицей одновременно
func mixedScriptLabel(domain string) bool {
	for label := range strings.SplitSeq(domain, ".") {
		var latin, cyr bool
		for _, r := range label {
			latin = latin || unicode.Is(unicode.Latin, r)
			cyr = cyr || unicode.Is(unicode.Cyrillic, r)
		}
		if latin && cyr {
			return true
		}
	}
	return false
}

// ValidatePassword - правила для нового пароля при регистрации
func ValidatePassword(s string) error {
	if s == "" {
		return ErrRequired
	}
	n := utf8.RuneCountInString(s)
	if n < passwordMinLen {
		return ErrPasswordTooShort
	}
	if n > passwordMaxLen {
		return ErrPasswordTooLong
	}

	var hasLetter, hasDigit bool
	for _, r := range s {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return ErrPasswordWeak
	}
	return nil
}

// ValidateLoginPassword - при логине только непустой и не длиннее 128, без правил сложности
func ValidateLoginPassword(s string) error {
	if s == "" {
		return ErrRequired
	}
	if utf8.RuneCountInString(s) > passwordMaxLen {
		return ErrPasswordTooLong
	}
	return nil
}

// ValidateName ожидает строку после NormalizeName
func ValidateName(s string) error {
	if s == "" {
		return ErrRequired
	}
	if utf8.RuneCountInString(s) > nameMaxLen {
		return ErrNameTooLong
	}

	afterLetter := false
	var last rune
	for _, r := range s {
		switch {
		case unicode.IsLetter(r):
			afterLetter = true
		case unicode.Is(unicode.M, r):
			if !afterLetter {
				return ErrNameFormat
			}
		case r == ' ' || r == '-' || r == '\'' || r == '.':
			if !afterLetter {
				return ErrNameFormat
			}
			afterLetter = false
		default:
			return ErrNameFormat
		}
		last = r
	}
	if !afterLetter && last != '.' {
		return ErrNameFormat
	}
	return nil
}

// NormalizeName обрезает пробелы по краям и схлопывает пробелы внутри
func NormalizeName(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// ValidateAge проверяет, что на момент вызова пользователю есть 18 лет
func ValidateAge(birthDate time.Time) error {
	return ValidateAgeAt(birthDate, time.Now())
}

// ValidateAgeAt проверяет минимальный возраст на дату now
func ValidateAgeAt(birthDate, now time.Time) error {
	birth := time.Date(birthDate.Year(), birthDate.Month(), birthDate.Day(), 0, 0, 0, 0, time.UTC)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if birth.AddDate(minAge, 0, 0).After(today) {
		return ErrUnderage
	}
	return nil
}

// ParseBirthDate разбирает YYYY-MM-DD; несуществующие даты отбрасывает time.Parse
func ParseBirthDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, ErrRequired
	}
	t, err := time.Parse(DateLayout, s)
	if err != nil {
		return time.Time{}, ErrDateFormat
	}
	return t, nil
}

func ValidateSex(s string) error {
	return oneOf(s, sexValues, ErrSex)
}

func ValidateSearchSex(s string) error {
	return oneOf(s, searchSexValues, ErrSearchSex)
}

func ValidateDatingGoal(s string) error {
	if s == "" {
		return ErrRequired
	}
	if !slices.Contains(model.DatingGoals, model.DatingGoal(s)) {
		return ErrDatingGoal
	}
	return nil
}

// ValidateAboutMe - описание необязательно, пустая строка допустима
func ValidateAboutMe(s string) error {
	if utf8.RuneCountInString(s) > aboutMeMaxLen {
		return ErrAboutMeTooLong
	}
	return nil
}

func ValidateEducation(s string) error {
	if !slices.Contains(model.Educations, model.Education(s)) {
		return ErrEducation
	}
	return nil
}

// ValidateAttitude - отношение к курению или алкоголю
func ValidateAttitude(s string) error {
	if !slices.Contains(model.Attitudes, model.Attitude(s)) {
		return ErrAttitude
	}
	return nil
}

// ValidateWork ожидает строку после strings.TrimSpace; пустая строка - очистка поля
func ValidateWork(s string) error {
	if utf8.RuneCountInString(s) > workMaxLen {
		return ErrWorkTooLong
	}
	return nil
}

// ValidateHeight - рост в сантиметрах
func ValidateHeight(h int) error {
	if h < model.MinHeight || h > model.MaxHeight {
		return ErrHeight
	}
	return nil
}

// ValidateSearchAge - граница возраста для поиска анкет
func ValidateSearchAge(age int) error {
	if age < model.MinSearchAge {
		return ErrSearchAgeTooLow
	}
	if age > model.MaxSearchAge {
		return ErrSearchAgeTooHigh
	}
	return nil
}

func ValidateSearchAgeRange(from, to int) error {
	if to < from {
		return ErrSearchAgeRange
	}
	return nil
}

// ValidateTags - теги необязательны, nil и пустой список допустимы
func ValidateTags(tags []string) error {
	if len(tags) > tagsMaxCount {
		return ErrTagsTooMany
	}
	seen := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		if t == "" || utf8.RuneCountInString(t) > tagMaxLen || strings.TrimSpace(t) == "" {
			return ErrTagInvalid
		}
		if _, ok := seen[t]; ok {
			return ErrTagsNotUniq
		}
		seen[t] = struct{}{}
	}
	return nil
}

func ValidatePhoto(p model.PhotoUpload) error {
	if len(p.Data) > PhotoMaxSize {
		return ErrPhotoTooLarge
	}
	if p.Ext == "" {
		return ErrPhotoFormat
	}
	return nil
}

// ValidatePhotoOrder - непустой список положительных id без повторов, не длиннее model.MaxPhotos
func ValidatePhotoOrder(ids []int64) error {
	if len(ids) == 0 || len(ids) > model.MaxPhotos {
		return ErrPhotoOrder
	}
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok || id <= 0 {
			return ErrPhotoOrder
		}
		seen[id] = struct{}{}
	}
	return nil
}

// PhotoExt определяет формат по содержимому файла
func PhotoExt(data []byte) string {
	return photoExtByContentType[http.DetectContentType(data)]
}

func oneOf(s string, allowed []string, errInvalid error) error {
	if s == "" {
		return ErrRequired
	}
	if slices.Contains(allowed, s) {
		return nil
	}
	return errInvalid
}
