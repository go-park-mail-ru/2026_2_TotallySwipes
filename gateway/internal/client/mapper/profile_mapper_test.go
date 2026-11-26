package mapper

import (
	"reflect"
	"testing"

	"dating-app/internal/model"
)

func TestProfilePatchPreservesClearAndOmittedFields(t *testing.T) {

	work := "Developer"
	tags := []string{}
	request, err := ProfilePatchToProto(7, &model.ProfilePatch{
		AboutMe: model.Optional[string]{Set: true},
		Work:    model.Optional[string]{Set: true, Value: &work},
		Tags:    &tags,
	})

	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(request.GetUpdateMask().GetPaths(), []string{"about_me", "work", "tags"}) {
		t.Fatalf("unexpected mask: %v", request.GetUpdateMask())
	}

	if request.GetFields().AboutMe != nil || request.GetFields().Name != nil || request.GetFields().GetWork() != work || len(request.GetFields().GetTags()) != 0 {
		t.Fatalf("unexpected patch: %v", request.GetFields())
	}
}

func TestSearchFilterRoundTrip(t *testing.T) {

	original := model.SearchFilter{Sex: model.SearchSexAll, AgeFrom: 18, AgeTo: 100}
	result, err := SearchFilterFromProto(SearchFilterToProto(original))

	if err != nil || *result != original {
		t.Fatalf("unexpected filter: %+v, %v", result, err)
	}

	if _, err := SearchFilterFromProto(nil); err == nil {
		t.Fatal("expected missing filter error")
	}
}

func TestSwipeMapping(t *testing.T) {

	if _, err := SwipeToProto(&model.Swipe{Action: "unknown"}); err == nil {
		t.Fatal("expected unknown action error")
	}

	if _, err := SwipeToProto(nil); err == nil {
		t.Fatal("expected missing swipe error")
	}
}
