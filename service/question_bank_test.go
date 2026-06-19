package service

import (
	"reflect"
	"testing"
)

func TestExtractAnswersWanjuan(t *testing.T) {
	raw := `{"code":1,"data":{"questions":[{"answer":"A#C"}]}}`
	got := extractAnswersFromJSON(raw, "data.questions.0.answer", "#")
	want := []string{"A", "C"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("answers=%v want %v", got, want)
	}
}

func TestExtractAnswersZError(t *testing.T) {
	raw := `{"code":1,"data":{"data":"正确"}}`
	got := extractAnswersFromJSON(raw, "data.data", "#")
	want := []string{"正确"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("answers=%v want %v", got, want)
	}
}

func TestExtractAnswersCustomArray(t *testing.T) {
	raw := `{"answers":["A","B","A",""]}`
	got := extractAnswersFromJSON(raw, "answers", "#")
	want := []string{"A", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("answers=%v want %v", got, want)
	}
}

func TestWanjuanQuestionType(t *testing.T) {
	cases := map[string]string{
		"单选题": "single",
		"多选题": "multiple",
		"判断题": "judge",
		"填空题": "completion",
		"简答题": "short",
	}
	for in, want := range cases {
		if got := wanjuanQuestionType(in); got != want {
			t.Fatalf("%s => %s want %s", in, got, want)
		}
	}
}

func TestBuildQuestionBankPayloadXHWL(t *testing.T) {
	setting := normalizeQuestionBankSetting(ApiQueSetting{Protocol: "xhwlgzs"})
	payload := buildQuestionBankPayload(setting, QuestionBankQuestion{
		Type:       "single choice",
		Content:    "1+1?",
		OptionsMap: map[string]string{"A": "1", "B": "2"},
	})
	if payload["value"] != "1+1?" {
		t.Fatalf("value=%v", payload["value"])
	}
	if payload["type"] != 0 {
		t.Fatalf("type=%v want 0", payload["type"])
	}
	options, ok := payload["options"].(map[string]string)
	if !ok || options["B"] != "2" {
		t.Fatalf("options=%#v", payload["options"])
	}
}

func TestApplyBearerAuth(t *testing.T) {
	setting := normalizeQuestionBankSetting(ApiQueSetting{Protocol: "xhwlgzs", Token: "abc"})
	payload := map[string]any{}
	endpoint := setting.Url
	headers := map[string]string{}
	applyAuth(setting, payload, &endpoint, headers)
	if headers["Authorization"] != "Bearer abc" {
		t.Fatalf("headers=%v", headers)
	}
	if _, ok := payload["Authorization"]; ok {
		t.Fatalf("bearer token should not be written to body: %v", payload)
	}
}
