package psychotest

import (
	"testing"

	"dating-app/internal/model"
	"dating-app/internal/psychotest"
)

func TestLoadEmbeddedTIPI(t *testing.T) {
	test, err := psychotest.Load()
	if err != nil {
		t.Fatal(err)
	}
	if test.ID != 1 || test.Methodology != model.MethodologyTIPI || test.Title == "" {
		t.Fatalf("header = %+v", test)
	}
	if len(test.AnswerOptions) != 7 || len(test.Questions) != 10 {
		t.Fatalf("options = %d, questions = %d", len(test.AnswerOptions), len(test.Questions))
	}
	for i, q := range test.Questions {
		if q.ID != int64(i+1) || q.OperationID != q.ID || q.Body == "" {
			t.Fatalf("question %d = %+v", i, q)
		}
	}
}
