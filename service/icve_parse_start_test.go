package service

import "testing"

func TestICVEParseStartRespPrefersDynamicWhenTypedProblemsAreEmpty(t *testing.T) {
	raw := `{"code":200,"data":{"id":"record-1","taskExamProblemRecordList":[{"id":"paper-1","question":{"title":"真实题干","typeName":"单选","dataJson":"[{\"Label\":0,\"SortOrder\":0,\"Content\":\"A\"},{\"Label\":1,\"SortOrder\":1,\"Content\":\"B\"}]","questionId":"qid-1"}}]}}`
	rec, err := icveParseStartResp(raw)
	if err != nil {
		t.Fatalf("parse start response: %v", err)
	}
	problems := rec.problems()
	if len(problems) != 1 {
		t.Fatalf("expected 1 problem, got %d", len(problems))
	}
	if problems[0].Stem != "真实题干" {
		t.Fatalf("dynamic parser did not recover stem: %#v", problems[0])
	}
	if len(problems[0].Options) != 2 {
		t.Fatalf("dynamic parser did not recover options: %#v", problems[0].Options)
	}
	if len(problems[0].RawFields) == 0 {
		t.Fatalf("dynamic parser should preserve raw fields")
	}
}
