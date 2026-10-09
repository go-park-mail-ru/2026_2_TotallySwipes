package dto

import (
	"bytes"
	"dating-app/internal/model"
	"dating-app/internal/validate"
	"encoding/json"
	"time"
)

type ProfileShortResponse struct {
	UserID        int64    `json:"user_id"`
	Name          *string  `json:"name"`
	Age           *int     `json:"age"`
	PhotoURL      *string  `json:"photo_url"`
	Missing       []string `json:"missing"`
}

func NewProfileShortResponse(p *model.ProfileShort) ProfileShortResponse {
	return ProfileShortResponse{
		UserID: p.UserID, Name: p.Name, Age: p.Age, PhotoURL: p.MainPhotoURL,
		Missing: p.Missing,
	}
}

type PhotoResponse struct {
	ID  int64  `json:"id"`
	URL string `json:"url"`
}

// PhotosResponse - актуальный список фото; photos[0] - главная
type PhotosResponse struct {
	Photos []PhotoResponse `json:"photos"`
}

func NewPhotosResponse(photos []model.Photo) PhotosResponse {
	return PhotosResponse{Photos: newPhotoResponses(photos)}
}

func newPhotoResponses(photos []model.Photo) []PhotoResponse {
	resp := make([]PhotoResponse, 0, len(photos))
	for _, p := range photos {
		resp = append(resp, PhotoResponse{ID: p.ID, URL: p.URL})
	}
	return resp
}

// ProfileResponse - анкета текущего пользователя. Незаполненные поля - null
type ProfileResponse struct {
	UserID        int64             `json:"user_id"`
	Name          *string           `json:"name"`
	BirthDate     *string           `json:"birth_date"`
	Age           *int              `json:"age"`
	Sex           *model.Sex        `json:"sex"`
	DatingGoal    *model.DatingGoal `json:"dating_goal"`
	AboutMe       *string           `json:"about_me"`
	SearchSex     *model.SearchSex  `json:"search_sex"`
	SearchAgeFrom *int              `json:"search_age_from"`
	SearchAgeTo   *int              `json:"search_age_to"`
	Tags          []string          `json:"tags"`
	Photos        []PhotoResponse   `json:"photos"`
	Missing       []string          `json:"missing"`
}

func NewProfileResponse(p *model.Profile, now time.Time) ProfileResponse {
	f := p.CurrentVersion.ProfileFields
	resp := ProfileResponse{
		UserID:        p.UserID,
		Name:          f.Name,
		Sex:           f.Sex,
		DatingGoal:    f.DatingGoal,
		AboutMe:       f.AboutMe,
		SearchSex:     f.SearchSex,
		SearchAgeFrom: f.SearchAgeFrom,
		SearchAgeTo:   f.SearchAgeTo,
		Tags:          make([]string, 0, len(p.Tags)),
		Photos:        newPhotoResponses(p.Photos),
		Missing:       p.Missing(),
	}
	if f.BirthDate != nil {
		date := f.BirthDate.Format(validate.DateLayout)
		age := model.AgeAt(*f.BirthDate, now)
		resp.BirthDate, resp.Age = &date, &age
	}
	for _, t := range p.Tags {
		resp.Tags = append(resp.Tags, t.Name)
	}
	return resp
}

// OptionalString отличает отсутствующее в JSON поле от явного null
type OptionalString struct {
	Set   bool
	Value *string
}

func (o *OptionalString) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(data, []byte("null")) {
		o.Value = nil
		return nil
	}
	return json.Unmarshal(data, &o.Value)
}

// UpdateProfileRequest - PATCH /profile/me: любое непустое подмножество полей.
// Отсутствующее поле не меняется. Обязательные поля анкеты очистить нельзя,
// about_me очищается через null, tags заменяются целиком
type UpdateProfileRequest struct {
	Name          *string        `json:"name"`
	BirthDate     *string        `json:"birth_date"`
	Sex           *string        `json:"sex"`
	DatingGoal    *string        `json:"dating_goal"`
	AboutMe       OptionalString `json:"about_me"`
	SearchSex     *string        `json:"search_sex"`
	SearchAgeFrom *int           `json:"search_age_from"`
	SearchAgeTo   *int           `json:"search_age_to"`
	Tags          *[]string      `json:"tags"`
}

// ToPatch нормализует и проверяет присланные поля. Пустая map - запрос корректен
func (r UpdateProfileRequest) ToPatch() (*model.ProfilePatch, map[string]string) {
	errs := make(map[string]string)
	patch := &model.ProfilePatch{Tags: r.Tags}

	if r.Name != nil {
		name := validate.NormalizeName(*r.Name)
		addErr(errs, "name", validate.ValidateName(name))
		patch.Name = &name
	}
	if r.BirthDate != nil {
		if birthDate, err := validate.ParseBirthDate(*r.BirthDate); err != nil {
			addErr(errs, "birth_date", err)
		} else {
			addErr(errs, "birth_date", validate.ValidateAge(birthDate))
			patch.BirthDate = &birthDate
		}
	}
	if r.Sex != nil {
		addErr(errs, "sex", validate.ValidateSex(*r.Sex))
		sex := model.Sex(*r.Sex)
		patch.Sex = &sex
	}
	if r.DatingGoal != nil {
		addErr(errs, "dating_goal", validate.ValidateDatingGoal(*r.DatingGoal))
		goal := model.DatingGoal(*r.DatingGoal)
		patch.DatingGoal = &goal
	}
	if r.AboutMe.Set {
		patch.AboutMeSet = true
		// Пустая строка - то же, что отсутствие описания
		if r.AboutMe.Value != nil && *r.AboutMe.Value != "" {
			addErr(errs, "about_me", validate.ValidateAboutMe(*r.AboutMe.Value))
			patch.AboutMe = r.AboutMe.Value
		}
	}
	if r.SearchSex != nil {
		addErr(errs, "search_sex", validate.ValidateSearchSex(*r.SearchSex))
		searchSex := model.SearchSex(*r.SearchSex)
		patch.SearchSex = &searchSex
	}
	if (r.SearchAgeFrom == nil) != (r.SearchAgeTo == nil) {
		errs["search_age_to"] = validate.ErrSearchAgePair.Error()
	} else if r.SearchAgeFrom != nil {
		fromErr := validate.ValidateSearchAge(*r.SearchAgeFrom)
		addErr(errs, "search_age_from", fromErr)
		if err := validate.ValidateSearchAge(*r.SearchAgeTo); err != nil {
			addErr(errs, "search_age_to", err)
		} else if fromErr == nil {
			addErr(errs, "search_age_to", validate.ValidateSearchAgeRange(*r.SearchAgeFrom, *r.SearchAgeTo))
		}
		patch.SearchAgeFrom, patch.SearchAgeTo = r.SearchAgeFrom, r.SearchAgeTo
	}
	if r.Tags != nil {
		addErr(errs, "tags", validate.ValidateTags(*r.Tags))
	}

	if len(errs) == 0 && patch.IsEmpty() {
		errs["profile"] = "нужно передать хотя бы одно поле"
	}
	return patch, errs
}
