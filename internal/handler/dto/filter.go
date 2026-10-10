package dto

import (
	"dating-app/internal/model"
	"dating-app/internal/validate"
)

type FilterResponse struct {
	Sex     model.SearchSex `json:"sex"`
	AgeFrom int             `json:"age_from"`
	AgeTo   int             `json:"age_to"`
}

func NewFilterResponse(f *model.SearchFilter) FilterResponse {
	return FilterResponse{Sex: f.Sex, AgeFrom: f.AgeFrom, AgeTo: f.AgeTo}
}

// SetFilterRequest - PUT /filters/me: фильтр передаётся целиком, все поля обязательны
type SetFilterRequest struct {
	Sex     *string `json:"sex"`
	AgeFrom *int    `json:"age_from"`
	AgeTo   *int    `json:"age_to"`
}

// ToModel проверяет фильтр. Пустая map - запрос корректен
func (r SetFilterRequest) ToModel() (model.SearchFilter, map[string]string) {
	errs := make(map[string]string)
	var f model.SearchFilter

	if r.Sex == nil {
		errs["sex"] = validate.ErrRequired.Error()
	} else {
		addErr(errs, "sex", validate.ValidateSearchSex(*r.Sex))
		f.Sex = model.SearchSex(*r.Sex)
	}
	fromOK := requiredSearchAge(errs, "age_from", r.AgeFrom)
	toOK := requiredSearchAge(errs, "age_to", r.AgeTo)
	if fromOK && toOK {
		f.AgeFrom, f.AgeTo = *r.AgeFrom, *r.AgeTo
		addErr(errs, "age_to", validate.ValidateSearchAgeRange(f.AgeFrom, f.AgeTo))
	}
	return f, errs
}

func requiredSearchAge(errs map[string]string, field string, age *int) bool {
	if age == nil {
		errs[field] = validate.ErrRequired.Error()
		return false
	}
	err := validate.ValidateSearchAge(*age)
	addErr(errs, field, err)
	return err == nil
}
