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
	emailMaxLen = 254

	passwordMinLen = 8
	passwordMaxLen = 128

	nameMaxLen = 64

	tagsMaxCount = 10
	tagMaxLen    = 30

	photosMinCount = 1
	photosMaxCount = 6
	PhotoMaxSize   = 5 << 20

	minAge       = 18  // нижняя граница в принципе на сервисе
	maxSearchAge = 100 // верхняя граница возраста в фильтре поиска

	DateLayout = "2006-01-02" // формат birth_date в запросах
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

	ErrSex          = errors.New("допустимые значения: male, female")
	ErrSearchSex    = errors.New("допустимые значения: male, female, all")
	ErrDatingIntent = errors.New("недопустимая цель знакомства")

	ErrSearchAgeTooLow  = errors.New("возраст для поиска должен быть не меньше 18")
	ErrSearchAgeTooHigh = errors.New("возраст для поиска должен быть не больше 100")
	ErrSearchAgeRange   = errors.New("верхняя граница возраста не может быть меньше нижней")

	ErrTagsTooMany = errors.New("не больше 10 тегов")
	ErrTagsNotUniq = errors.New("теги не должны повторяться")
	ErrTagInvalid  = errors.New("тег должен быть от 1 до 30 символов и не состоять только из пробелов")

	ErrNotInteger = errors.New("должно быть целым числом")

	ErrPhotosCount   = errors.New("нужно от 1 до 6 фотографий")
	ErrPhotoTooLarge = errors.New("фото должно быть не больше 5 МБ")
	ErrPhotoFormat   = errors.New("фото должно быть в формате JPEG, PNG или WebP")
)

// emailRe - регулярка из HTML5 (input type="email")
var emailRe = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@" +
	"[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?" +
	"(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$")

var photoExtByContentType = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

var (
	sexValues       = []string{"male", "female"}
	searchSexValues = []string{"male", "female", "all"}
)

// ValidateEmail ожидает уже обрезанную по краям строку
func ValidateEmail(s string) error {
	if s == "" {
		return ErrRequired
	}
	if utf8.RuneCountInString(s) > emailMaxLen {
		return ErrEmailTooLong
	}
	if !emailRe.MatchString(s) {
		return ErrEmailFormat
	}
	return nil
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

// ValidateLoginPassword - при логине только непустой и не длиннее 128,
// правила сложности не проверяем
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

// ValidateAgeAt checks the minimum age using the supplied reference date.
func ValidateAgeAt(birthDate, now time.Time) error {
	birth := time.Date(birthDate.Year(), birthDate.Month(), birthDate.Day(), 0, 0, 0, 0, time.UTC)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if birth.AddDate(minAge, 0, 0).After(today) {
		return ErrUnderage
	}
	return nil
}

// ParseBirthDate разбирает дату YYYY-MM-DD. time.Parse сам отбрасывает
// несуществующие даты вроде 2005-02-30
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

func ValidateDatingIntent(s string) error {
	if s == "" {
		return ErrRequired
	}
	if _, ok := model.DatingGoalByIntent[s]; !ok {
		return ErrDatingIntent
	}
	return nil
}

// ValidateSearchAge - граница возраста для поиска анкет
func ValidateSearchAge(age int) error {
	if age < minAge {
		return ErrSearchAgeTooLow
	}
	if age > maxSearchAge {
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

func ValidatePhotos(photos []model.PhotoUpload) error {
	if len(photos) < photosMinCount || len(photos) > photosMaxCount {
		return ErrPhotosCount
	}
	for _, p := range photos {
		if len(p.Data) > PhotoMaxSize {
			return ErrPhotoTooLarge
		}
		if p.Ext == "" {
			return ErrPhotoFormat
		}
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
