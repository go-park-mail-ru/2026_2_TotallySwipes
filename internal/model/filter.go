package model

type SearchSex string

const (
	SearchSexMale   SearchSex = "male"
	SearchSexFemale SearchSex = "female"
	SearchSexAll    SearchSex = "all"
)

const (
	MinSearchAge = 18
	MaxSearchAge = 100
)

// SearchFilter - кого пользователь хочет видеть в ленте; возраст от AgeFrom до AgeTo включительно
type SearchFilter struct {
	Sex     SearchSex
	AgeFrom int
	AgeTo   int
}

// DefaultSearchFilter действует, пока пользователь не задал свой фильтр
var DefaultSearchFilter = SearchFilter{Sex: SearchSexAll, AgeFrom: MinSearchAge, AgeTo: MaxSearchAge}
