package dto

type TestQuestion struct {
	ID   string `json:"id"`
	Body string `json:"body"`
}

type TestOption struct {
	Value int    `json:"value"`
	Label string `json:"label"`
}

type CurrentTestResponse struct {
	TestID        string         `json:"test_id"`
	Title         string         `json:"title"`
	Instructions  string         `json:"instructions"`
	AnswerOptions []TestOption   `json:"answer_options"`
	Questions     []TestQuestion `json:"questions"`
}
