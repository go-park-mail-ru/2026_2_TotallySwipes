package dto

import (
	"bytes"
	"dating-app/internal/model"
	"dating-app/internal/validate"
	"encoding/json"
	"strings"
	"time"
)

type ProfileShortResponse struct {
	UserID   int64    `json:"user_id"`
	Name     *string  `json:"name"`
	Age      *int     `json:"age"`
	PhotoURL *string  `json:"photo_url"`
	Missing  []string `json:"missing"`
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

// ReorderPhotosRequest - PUT /profile/me/photos/order: id всех фото в новом порядке, первое - главное
type ReorderPhotosRequest struct {
	PhotoIDs []int64 `json:"photo_ids"`
}

func (r ReorderPhotosRequest) Validate() map[string]string {
	errs := make(map[string]string)
	addErr(errs, "photo_ids", validate.ValidatePhotoOrder(r.PhotoIDs))
	return errs
}

// ProfileResponse - анкета текущего пользователя. Незаполненные поля - null
type ProfileResponse struct {
	UserID     int64             `json:"user_id"`
	Name       *string           `json:"name"`
	BirthDate  *string           `json:"birth_date"`
	Age        *int              `json:"age"`
	Sex        *model.Sex        `json:"sex"`
	DatingGoal *model.DatingGoal `json:"dating_goal"`
	AboutMe    *string           `json:"about_me"`
	Education  *model.Education  `json:"education"`
	Work       *string           `json:"work"`
	Smoking    *model.Attitude   `json:"smoking"`
	Alcohol    *model.Attitude   `json:"alcohol"`
	Height     *int              `json:"height"`
	Tags       []string          `json:"tags"`
	Photos     []PhotoResponse   `json:"photos"`
	Missing    []string          `json:"missing"`
}

func NewProfileResponse(p *model.Profile, now time.Time) ProfileResponse {
	f := p.CurrentVersion.ProfileFields
	resp := ProfileResponse{
		UserID:     p.UserID,
		Name:       f.Name,
		Sex:        f.Sex,
		DatingGoal: f.DatingGoal,
		AboutMe:    f.AboutMe,
		Education:  f.Education,
		Work:       f.Work,
		Smoking:    f.Smoking,
		Alcohol:    f.Alcohol,
		Height:     f.Height,
		Tags:       make([]string, 0, len(p.Tags)),
		Photos:     newPhotoResponses(p.Photos),
		Missing:    p.Missing(),
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

// Optional отличает отсутствующее в JSON поле от явного null
type Optional[T any] struct {
	Set   bool
	Value *T
}

func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(data, []byte("null")) {
		o.Value = nil
		return nil
	}
	return json.Unmarshal(data, &o.Value)
}

// UpdateProfileRequest - PATCH /profile/me: отсутствующее поле не меняется, необязательные чистятся через null, tags заменяются целиком
type UpdateProfileRequest struct {
	Name       *string          `json:"name"`
	BirthDate  *string          `json:"birth_date"`
	Sex        *string          `json:"sex"`
	DatingGoal *string          `json:"dating_goal"`
	AboutMe    Optional[string] `json:"about_me"`
	Education  Optional[string] `json:"education"`
	Work       Optional[string] `json:"work"`
	Smoking    Optional[string] `json:"smoking"`
	Alcohol    Optional[string] `json:"alcohol"`
	Height     Optional[int]    `json:"height"`
	Tags       *[]string        `json:"tags"`
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
	patch.AboutMe = optionalText(errs, "about_me", r.AboutMe, validate.ValidateAboutMe)
	patch.Work = optionalText(errs, "work", trimmed(r.Work), validate.ValidateWork)
	patch.Education = optionalEnum[model.Education](errs, "education", r.Education, validate.ValidateEducation)
	patch.Smoking = optionalEnum[model.Attitude](errs, "smoking", r.Smoking, validate.ValidateAttitude)
	patch.Alcohol = optionalEnum[model.Attitude](errs, "alcohol", r.Alcohol, validate.ValidateAttitude)
	patch.Height = model.Optional[int](r.Height)
	if r.Height.Value != nil {
		addErr(errs, "height", validate.ValidateHeight(*r.Height.Value))
	}
	if r.Tags != nil {
		addErr(errs, "tags", validate.ValidateTags(*r.Tags))
	}

	if len(errs) == 0 && patch.IsEmpty() {
		errs["profile"] = "нужно передать хотя бы одно поле"
	}
	return patch, errs
}

func trimmed(o Optional[string]) Optional[string] {
	if o.Value != nil {
		v := strings.TrimSpace(*o.Value)
		o.Value = &v
	}
	return o
}

// optionalText проверяет текст; пустая строка, как и null, очищает поле
func optionalText(errs map[string]string, field string, o Optional[string], check func(string) error) model.Optional[string] {
	if o.Value == nil || *o.Value == "" {
		return model.Optional[string]{Set: o.Set}
	}
	addErr(errs, field, check(*o.Value))
	return model.Optional[string](o)
}

func optionalEnum[T ~string](errs map[string]string, field string, o Optional[string], check func(string) error) model.Optional[T] {
	if o.Value == nil {
		return model.Optional[T]{Set: o.Set}
	}
	addErr(errs, field, check(*o.Value))
	v := T(*o.Value)
	return model.Optional[T]{Set: true, Value: &v}
}
