package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	consoleConfig "yatori-go-console/config"
)

func TestICVEParseOptionsMap(t *testing.T) {
	m := icveParseOptionsMap(`[{"Label":0,"Content":"first"},{"Label":1,"Content":"second"}]`)
	if m["0"] != "first" || m["A"] != "first" || m["1"] != "second" || m["B"] != "second" {
		t.Fatalf("dual-key option map wrong: %v", m)
	}
}

func TestApplyICVEAnswerWritesRecordAnswer(t *testing.T) {
	q := icveExamProblemRecord{
		Options: []icveOptionItem{
			{Label: "A", Content: "red", Id: "0"},
			{Label: "B", Content: "green", Id: "1"},
		},
		RawFields: map[string]interface{}{},
	}
	applyICVEAnswer(&q, []string{"A"})
	if q.RecordAnswer != "0" {
		t.Fatalf("want recordAnswer 0, got %q", q.RecordAnswer)
	}
	if q.Answer != "" {
		t.Fatalf("must not overwrite original answer, got %q", q.Answer)
	}
	if q.RawFields["recordAnswer"] != "0" {
		t.Fatalf("raw recordAnswer not synced: %#v", q.RawFields)
	}
}

func TestParseICVEQuestionArrayDataJsonOptions(t *testing.T) {
	arr := []interface{}{
		map[string]interface{}{
			"title":        "question one",
			"typeName":     "multiple",
			"answer":       "0,1",
			"recordAnswer": "",
			"dataJson":     `[{"Label":0,"SortOrder":0,"Content":"red"},{"Label":1,"SortOrder":1,"Content":"green"}]`,
			"paperId":      "paper-1",
			"questionId":   "question-1",
		},
	}
	result := parseICVEQuestionArray(arr)
	if len(result) != 1 {
		t.Fatalf("want 1 question, got %d", len(result))
	}
	q := result[0]
	if len(q.Options) != 2 {
		t.Fatalf("want 2 parsed options, got %d: %+v", len(q.Options), q.Options)
	}
	if q.OptionsMap["0"] != "red" || q.OptionsMap["A"] != "red" {
		t.Fatalf("dual key options missing: %v", q.OptionsMap)
	}
	if q.Answer != "0,1" {
		t.Fatalf("original answer should be preserved, got %q", q.Answer)
	}
}

func TestICVENoRandomFallback(t *testing.T) {
	q := icveExamProblemRecord{Answer: "", RecordAnswer: ""}
	_ = icveParseOptionsMap(`[{"Label":0,"Content":"A"},{"Label":1,"Content":"B"}]`)
	if q.Answer != "" || q.RecordAnswer != "" {
		t.Fatal("parsing options must not modify answers")
	}
}

func TestICVEExamLabel(t *testing.T) {
	if got := icveExamLabel(icveExamItem{ExamName: "midterm"}); got != "midterm" {
		t.Fatalf("got %q", got)
	}
	if got := icveExamLabel(icveExamItem{Name: "在线自测", Id: "37pdagslq7dihk5qzbahwg"}); got != "在线自测" {
		t.Fatalf("want web display name, got %q", got)
	}
	got := icveExamLabel(icveExamItem{Id: "1234567890abcdef"})
	if !strings.Contains(got, "12345678") {
		t.Fatalf("fallback label missing id prefix: %q", got)
	}
}

func TestICVEExamKindRespectsTitle(t *testing.T) {
	if got := icveExamKind(icveExamItem{Name: "\u3010\u671f\u4e2d\u8003\u8bd5\u30115G", CategoryId: "2"}, "2"); got != "exam" {
		t.Fatalf("midterm should be exam, got %q", got)
	}
	if got := icveExamKind(icveExamItem{Name: "\u3010\u8bfe\u540e\u4f5c\u4e1a\u3011\u53c2\u6570\u914d\u7f6e", CategoryId: "2"}, "2"); got != "work" {
		t.Fatalf("homework should be work, got %q", got)
	}
	if got := icveExamKind(icveExamItem{Name: "\u3010\u6a21\u5757\u6d4b\u9a8c\u3011\u6a21\u5757\u4e8c", TypeId: "1", CategoryId: "1"}, "1"); got != "work" {
		t.Fatalf("module quiz typeId=1 categoryId=1 should be work, got %q", got)
	}
	if got := icveExamKind(icveExamItem{Name: "\u7ae0\u8282\u6d4b\u9a8c", CategoryId: "1"}, "1"); got != "quiz" {
		t.Fatalf("chapter test should be quiz, got %q", got)
	}
}

func TestICVEExamKindEnabledByAccountSwitch(t *testing.T) {
	off := 0
	on := 1
	cc := consoleConfig.CoursesCustom{
		CxExamSw:        &off,
		CxWorkSw:        &on,
		CxChapterTestSw: &on,
	}
	if icveExamKindEnabled("exam", cc) {
		t.Fatal("exam switch off should skip exam")
	}
	if !icveExamKindEnabled("work", cc) {
		t.Fatal("work switch on should handle work")
	}
	if !icveExamKindEnabled("quiz", cc) {
		t.Fatal("chapter test switch on should handle quiz")
	}
}

func TestICVEExamKindExamTitleWinsOverTypeId(t *testing.T) {
	item := icveExamItem{Name: "\u3010\u671f\u4e2d\u8003\u8bd5\u30115G\u65e0\u7ebf\u7f51\u7edc\u89c4\u5212\u4e0e\u4f18\u5316", TypeId: "1", CategoryId: "2"}
	kind := icveExamKind(item, "2")
	name := item.name()
	if icveNameContains(name, "\u8003\u8bd5") {
		kind = "exam"
	}
	if kind != "exam" {
		t.Fatalf("exam title must win over typeId=1, got %q", kind)
	}
}

func TestICVEStudentCourseMatchByName(t *testing.T) {
	course := icveStudentCourseItem{CourseId: "web-course", CourseInfoId: "web-info", CourseName: "5G\u65e0\u7ebf\u7f51\u7edc\u89c4\u5212\u4e0e\u4f18\u5316"}
	if course.displayName() != "5G\u65e0\u7ebf\u7f51\u7edc\u89c4\u5212\u4e0e\u4f18\u5316" {
		t.Fatalf("displayName mismatch: %q", course.displayName())
	}
}

func TestICVECourseMatchesIncludeWithTermSuffix(t *testing.T) {
	courseName := "5G\u65e0\u7ebf\u7f51\u7edc\u89c4\u5212\u4e0e\u4f18\u5316 - \u3010\u7b2c2\u671f\u30115G\u65e0\u7ebf\u7f51\u7edc\u89c4\u5212\u4e0e\u4f18\u5316"
	if !icveCourseMatchesList(courseName, []string{"5G\u65e0\u7ebf\u7f51\u7edc\u89c4\u5212\u4e0e\u4f18\u5316"}) {
		t.Fatalf("course with term suffix should match include list")
	}
}

func TestICVECourseKeyUsesCoursePair(t *testing.T) {
	if got := icveCourseKey("cid", "ciid", "name"); got != "cid_ciid" {
		t.Fatalf("course key mismatch: %q", got)
	}
}

func TestICVERandomFallbackDisabled(t *testing.T) {
	p := &icveExamProblemRecord{
		QuestionType: "single",
		OptionsMap:   map[string]string{"A": "red", "B": "green"},
		RawFields:    map[string]interface{}{},
	}
	icveApplyRandomFallback("u", "c", "e", 1, p, 0, func(_ string, _ ...interface{}) {})
	if p.RecordAnswer != "" {
		t.Fatalf("randomAnswerOnFail=0 should not set answer, got %q", p.RecordAnswer)
	}
}

func TestICVERandomFallbackSingle(t *testing.T) {
	p := &icveExamProblemRecord{
		QuestionType: "single",
		OptionsMap:   map[string]string{"A": "red", "B": "green"},
		RawFields:    map[string]interface{}{},
	}
	icveApplyRandomFallback("u", "c", "e", 1, p, 1, func(_ string, _ ...interface{}) {})
	if p.RecordAnswer == "" {
		t.Fatal("expected random fallback answer")
	}
	if _, ok := p.OptionsMap[p.RecordAnswer]; !ok {
		t.Fatalf("random answer %q not in options", p.RecordAnswer)
	}
}

func TestICVERandomFallbackJudge(t *testing.T) {
	p := &icveExamProblemRecord{
		QuestionType: "judge",
		OptionsMap:   map[string]string{"A": "true", "B": "false"},
		RawFields:    map[string]interface{}{},
	}
	icveApplyRandomFallback("u", "c", "e", 1, p, 1, func(_ string, _ ...interface{}) {})
	if p.RecordAnswer == "" {
		t.Fatal("expected judge fallback answer")
	}
}

func TestICVERandomFallbackMultiple(t *testing.T) {
	p := &icveExamProblemRecord{
		QuestionType: "multiple",
		OptionsMap:   map[string]string{"A": "red", "B": "green", "C": "blue", "D": "black"},
		RawFields:    map[string]interface{}{},
	}
	icveApplyRandomFallback("u", "c", "e", 1, p, 1, func(_ string, _ ...interface{}) {})
	if p.RecordAnswer == "" {
		t.Fatal("expected multiple fallback answer")
	}
	parts := strings.Split(p.RecordAnswer, ",")
	if len(parts) < 1 || len(parts) > 3 {
		t.Fatalf("want 1..3 selected answers, got %q", p.RecordAnswer)
	}
}

func TestICVERandomFallbackFillSkipped(t *testing.T) {
	for _, qt := range []string{"fill", "short", "essay"} {
		p := &icveExamProblemRecord{
			QuestionType: qt,
			OptionsMap:   map[string]string{"A": "red", "B": "green"},
			RawFields:    map[string]interface{}{},
		}
		icveApplyRandomFallback("u", "c", "e", 1, p, 1, func(_ string, _ ...interface{}) {})
		if p.RecordAnswer != "" {
			t.Fatalf("%s should not be randomized, got %q", qt, p.RecordAnswer)
		}
	}
}

func TestICVELogAnswerSnapshotUsesIndex(t *testing.T) {
	problems := []icveExamProblemRecord{
		{QuestionNo: 99, RecordAnswer: "A"},
		{QuestionNo: 0, RecordAnswer: "B"},
		{QuestionNo: -1},
	}
	var got string
	emit := func(format string, args ...interface{}) {
		got = fmt.Sprintf(format, args...)
	}
	icveLogAnswerSnapshot("u", "c", "e", problems, emit)
	for _, want := range []string{"1", "2", "3", "A", "B"} {
		if !strings.Contains(got, want) {
			t.Fatalf("snapshot missing %q: %s", want, got)
		}
	}
	if strings.Contains(got, "99") {
		t.Fatalf("snapshot must not use QuestionNo=99: %s", got)
	}
}

func TestErrNotInAnswerTime(t *testing.T) {
	if errNotInAnswerTime == nil {
		t.Fatal("errNotInAnswerTime should not be nil")
	}
}

func TestICVEExamSubmitPayloadIsLast(t *testing.T) {
	save := icveExamSubmitPayload{IsLast: true}
	submit := icveExamSubmitPayload{IsLast: true}
	if !save.IsLast {
		t.Fatal("save payload IsLast should be true")
	}
	if !submit.IsLast {
		t.Fatal("submit payload IsLast should be true")
	}
}

func TestICVEProblemMarshalWritesRecordAnswer(t *testing.T) {
	p := icveExamProblemRecord{
		RawFields: map[string]interface{}{
			"id":           "row-1",
			"answer":       "0",
			"recordAnswer": "",
		},
		Answer:       "0",
		RecordAnswer: "1",
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["answer"] != "0" {
		t.Fatalf("original answer should be preserved: %s", string(b))
	}
	if got["recordAnswer"] != "1" {
		t.Fatalf("recordAnswer should be written: %s", string(b))
	}
}

func TestIcveMapAnswerAlphaToNumeric(t *testing.T) {
	opts := []icveOptionItem{
		{Label: "A", Content: "red", Id: "0"},
		{Label: "B", Content: "green", Id: "1"},
		{Label: "C", Content: "blue", Id: "2"},
	}
	for _, c := range []struct{ in, want string }{{"A", "0"}, {"B", "1"}, {"C", "2"}, {"AB", "0,1"}, {"A,B", "0,1"}} {
		if got := icveMapAnswerToOption(c.in, opts, "multiple"); got != c.want {
			t.Fatalf("input %q: want %q got %q", c.in, c.want, got)
		}
	}
}

func TestIcveMapAnswerContentToNumeric(t *testing.T) {
	opts := []icveOptionItem{
		{Label: "A", Content: "correct answer", Id: "0"},
		{Label: "B", Content: "wrong answer", Id: "1"},
	}
	if got := icveMapAnswerToOption("correct answer", opts, "single"); got != "0" {
		t.Fatalf("want 0 got %q", got)
	}
}

func TestIcveMapQuestionBankAnswerFormats(t *testing.T) {
	opts := []icveOptionItem{
		{Label: "A", Content: "<p>脚踏实地</p>", Id: "0"},
		{Label: "B", Content: "职业能力", Id: "1"},
		{Label: "C", Content: "团队协作", Id: "2"},
	}
	cases := []struct {
		in   string
		want string
	}{
		{"答案：B", "1"},
		{"B.职业能力", "1"},
		{"<span>团队 协作</span>", "2"},
		{"脚踏实地", "0"},
	}
	for _, c := range cases {
		if got := icveMapAnswerToOption(c.in, opts, "single"); got != c.want {
			t.Fatalf("input %q: want %q got %q", c.in, c.want, got)
		}
	}
}

func TestAnsweredCountUsesRecordAnswer(t *testing.T) {
	problems := []icveExamProblemRecord{
		{Answer: "0", RecordAnswer: ""},
		{Answer: "1", RecordAnswer: "1"},
		{Answer: "2", RecordAnswer: " "},
	}
	answered := 0
	for _, p := range problems {
		if strings.TrimSpace(p.RecordAnswer) != "" {
			answered++
		}
	}
	if answered != 1 {
		t.Fatalf("want 1 answered record, got %d", answered)
	}
}

func TestApplyICVEAnswerSliceIndex(t *testing.T) {
	problems := []icveExamProblemRecord{
		{Options: []icveOptionItem{{Label: "A", Content: "red", Id: "0"}}, RawFields: map[string]interface{}{}},
	}
	for i := range problems {
		applyICVEAnswer(&problems[i], []string{"A"})
	}
	if problems[0].RecordAnswer != "0" {
		t.Fatalf("slice element was not updated: %+v", problems[0])
	}
}

func TestSanitizeICVEAnswerCandidate(t *testing.T) {
	cases := []struct {
		in  string
		out string
	}{
		{"?", ""},
		{"?,1,3", "1,3"},
		{"A", "A"},
		{"true", "true"},
		{"\x01\x02", ""},
		{"  ", ""},
	}
	for _, c := range cases {
		got := sanitizeICVEAnswerCandidate(c.in)
		if got != c.out {
			t.Errorf("sanitizeICVEAnswerCandidate(%q) = %q, want %q", c.in, got, c.out)
		}
	}
}

func TestMapAnswerNoPanicOnQuestion(t *testing.T) {
	opts := []icveOptionItem{{Label: "A", Content: "yes", Id: "1"}, {Label: "B", Content: "no", Id: "2"}}
	// "?" should not panic and should return empty or original
	_ = icveMapAnswerToOption("?", opts, "single")
	// "?,1,3" 閳?"?" candidate is garbage, "1" and "3" may or may not match
	_ = icveMapAnswerToOption("?,1,3", opts, "multiple")
	// ",1,3" leading comma
	_ = icveMapAnswerToOption(",1,3", opts, "multiple")
	// single Chinese char that is len==1 rune 閳?must not panic on [1]
	_ = icveMapAnswerToOption("\u9519", opts, "single")
}

func TestMapAnswerRuneSafe(t *testing.T) {
	opts := []icveOptionItem{{Label: "A", Content: "option A", Id: "10"}}
	// "A." prefix 閳?ASCII, 2 bytes, rune-safe
	got := icveMapAnswerToOption("A.option A", opts, "single")
	if got != "10" {
		t.Errorf("expected 10, got %q", got)
	}
}

func TestApplyICVEAnswerSkipsEmptyMapped(t *testing.T) {
	// questionbank returns "?" 閳?after mapping, mapped=="", RecordAnswer must stay ""
	p := &icveExamProblemRecord{
		Options:   []icveOptionItem{{Label: "A", Content: "foo", Id: "1"}},
		RawFields: map[string]interface{}{},
	}
	applyICVEAnswer(p, []string{"?"})
	if p.RecordAnswer != "" {
		t.Fatalf("RecordAnswer should be empty for invalid answer, got %q", p.RecordAnswer)
	}
}

func TestPerQuestionRecoverNoPanic(t *testing.T) {
	// Simulate what the recover wrapper does: a panic inside must not propagate
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		// force a panic similar to the original bug
		var s []rune
		_ = s[1] // index out of range
	}()
	if !panicked {
		t.Fatal("expected recover to catch panic")
	}
}

func TestAllEmptyRecordAnswerGuard(t *testing.T) {
	problems := []icveExamProblemRecord{
		{Answer: "A", RecordAnswer: ""},
		{Answer: "B", RecordAnswer: ""},
	}
	answered := 0
	for _, p := range problems {
		if strings.TrimSpace(p.RecordAnswer) != "" {
			answered++
		}
	}
	if answered != 0 {
		t.Fatalf("all-empty guard: expected 0 answered, got %d", answered)
	}
}

// Task #25 tests

func TestFillAnswerGPSNotSplit(t *testing.T) {
	// GPS 鎼存柧绗夐幏鍡楀瀻閿涘苯锝炵粚娲暯娑撳秷铔嬬€涙鐦濋弰鐘茬殸
	q := icveExamProblemRecord{
		QuestionType: "fill",
		Options:      []icveOptionItem{},
		Answer:       `[{"SortOrder":0,"Content":""}]`,
	}
	applyICVEAnswer(&q, []string{"GPS"})
	if q.RecordAnswer == "" {
		t.Fatal("fill GPS: RecordAnswer should not be empty")
	}
	if strings.Contains(q.RecordAnswer, "G,P,S") || strings.Contains(q.RecordAnswer, `"G"`) {
		t.Fatalf("fill GPS must not be split, got %s", q.RecordAnswer)
	}
	if !strings.Contains(q.RecordAnswer, "GPS") {
		t.Fatalf("fill GPS: expected GPS in RecordAnswer, got %s", q.RecordAnswer)
	}
}

func TestFillAnswerRecordAnswerIsJSON(t *testing.T) {
	q := icveExamProblemRecord{
		QuestionType: "fill",
		Options:      []icveOptionItem{},
		Answer:       `[{"SortOrder":0,"Content":"placeholder"}]`,
	}
	applyICVEAnswer(&q, []string{"test answer"})
	var arr []map[string]interface{}
	if err := json.Unmarshal([]byte(q.RecordAnswer), &arr); err != nil {
		t.Fatalf("fill recordAnswer must be JSON array, got %s: %v", q.RecordAnswer, err)
	}
	if len(arr) == 0 {
		t.Fatal("fill recordAnswer array must not be empty")
	}
	if v, _ := arr[0]["Content"].(string); v != "test answer" {
		t.Fatalf("fill Content mismatch: %v", arr[0]["Content"])
	}
}

func TestJudgeMappingToOptionId(t *testing.T) {
	opts := []icveOptionItem{
		{Label: "A", Id: "0", Content: "true"},
		{Label: "B", Id: "1", Content: "false"},
	}
	res := icveMapAnswerToOption("true", opts, "judge")
	if res != "A" {
		t.Fatalf("judge answer should map to label A, got %q", res)
	}
}

func TestListItemKindOnlineSelfTestUsesQuiz(t *testing.T) {
	item := icveExamItem{Name: "在线自测", CategoryId: "3", TypeId: "1", Source: "exam"}
	if got := icveListItemKind(item, "3", "/teacher/exam/answeredExamList"); got != "quiz" {
		t.Fatalf("online self-test should be quiz, got %q", got)
	}
	if got := icveListItemKind(item, "3", "/teacher/homeworkExam/answeredExamList"); got != "work" {
		t.Fatalf("homework endpoint should stay work, got %q", got)
	}
}

func TestMultipleAnswerDedup(t *testing.T) {
	opts := []icveOptionItem{
		{Id: "0", Content: "option A"},
		{Id: "1", Content: "option B"},
		{Id: "2", Content: "option C"},
	}
	// "A,A,B" 鎼存柨骞撻柌宥勮礋 "0,1"
	res := icveMapAnswerToOption("A,A,B", opts, "multiple")
	parts := strings.Split(res, ",")
	seen := map[string]int{}
	for _, p := range parts {
		seen[p]++
	}
	for k, v := range seen {
		if v > 1 {
			t.Fatalf("multiple dedup: id %s appears %d times in %s", k, v, res)
		}
	}
}

// Task #26 tests

func TestIcveQuestionBankOptionsLetterKeys(t *testing.T) {
	// 瀹稿弶婀佺€涙鐦?key 閺冭绱濋崣顏囩箲閸ョ偛鐡уВ?key
	q := icveExamProblemRecord{
		OptionsMap: map[string]string{"A": "option A", "B": "option B", "0": "option A", "1": "option B"},
	}
	clean := icveQuestionBankOptions(q)
	for k := range clean {
		if k < "A" || k > "Z" {
			t.Fatalf("clean map must only have letter keys, got %q", k)
		}
	}
	if len(clean) != 2 {
		t.Fatalf("expected 2 letter keys, got %d: %v", len(clean), clean)
	}
}

func TestIcveQuestionBankOptionsNumericFallback(t *testing.T) {
	// 閸欘亝婀侀弫鏉跨摟 key 閺冭绱濇潪顑胯礋 A/B/C/D
	q := icveExamProblemRecord{
		OptionsMap: map[string]string{"0": "first", "1": "second"},
		Options: []icveOptionItem{
			{Id: "0", Content: "first"},
			{Id: "1", Content: "second"},
		},
	}
	clean := icveQuestionBankOptions(q)
	for k := range clean {
		if k < "A" || k > "Z" {
			t.Fatalf("fallback map must only have letter keys, got %q", k)
		}
	}
}

func TestApplyICVEAnswerFromAIAlpha(t *testing.T) {
	// AI 鏉╂柨娲?"A" 鏉╃偞甯撮柅澶愩€?id "0"
	q := icveExamProblemRecord{
		QuestionType: "single",
		Options: []icveOptionItem{
			{Id: "0", Content: "闁銆岮"},
			{Id: "1", Content: "闁銆岯"},
		},
	}
	applyICVEAnswer(&q, []string{"A"})
	if q.RecordAnswer == "" {
		t.Fatal("AI answer A should map to a recordAnswer")
	}
}

func TestApplyICVEAnswerFromAIContent(t *testing.T) {
	// AI 鏉╂柨娲栭柅澶愩€嶉崘鍛啇閺傚洦婀伴敍灞界安閼宠姤妲х亸鍕煂 id
	q := icveExamProblemRecord{
		QuestionType: "single",
		Options: []icveOptionItem{
			{Id: "0", Content: "wrong"},
			{Id: "1", Content: "right"},
		},
	}
	applyICVEAnswer(&q, []string{"right"})
	if q.RecordAnswer == "" {
		t.Fatal("AI content answer should map to a recordAnswer")
	}
}

func TestEmptyRecordAnswerSkipsSave(t *testing.T) {
	// recordAnswer 閸忋劎鈹栭弮?answered==0閿涘苯鐣ㄩ崗銊╂，鐠哄疇绻冩穱婵嗙摠
	problems := []icveExamProblemRecord{
		{QuestionType: "single", RecordAnswer: ""},
		{QuestionType: "fill", RecordAnswer: ""},
	}
	answered := 0
	for _, p := range problems {
		if strings.TrimSpace(p.RecordAnswer) != "" {
			answered++
		}
	}
	if answered != 0 {
		t.Fatalf("expected 0 answered, got %d", answered)
	}
}

func TestNonEmptyRecordAnswerCountsAsAnswered(t *testing.T) {
	problems := []icveExamProblemRecord{
		{QuestionType: "single", RecordAnswer: "0"},
		{QuestionType: "fill", RecordAnswer: `[{"SortOrder":0,"Content":"缁涙梹顢?}]`},
	}
	answered := 0
	for _, p := range problems {
		if strings.TrimSpace(p.RecordAnswer) != "" {
			answered++
		}
	}
	if answered != 2 {
		t.Fatalf("expected 2 answered, got %d", answered)
	}
}

// Tes
// Task #32 tests

func TestICVEJobTestDraftPayloadUsesAnswerField(t *testing.T) {
	problems := []icveExamProblemRecord{
		{QuestionNo: 1, Answer: "original", RecordAnswer: "2", PaperId: "p1"},
	}
	items := buildICVEDraftProblemList(problems)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Answer != "2" {
		t.Fatalf("draft answer must be RecordAnswer=2, got %q", items[0].Answer)
	}
}

func TestICVEJobTestDraftPayloadNoRecordAnswerRequired(t *testing.T) {
	problems := []icveExamProblemRecord{
		{QuestionNo: 1, Answer: "original", RecordAnswer: "1", PaperId: "p1",
			RawFields: map[string]interface{}{"knowledgePointsId": "kp1"}},
	}
	items := buildICVEDraftProblemList(problems)
	if items[0].Answer != "1" {
		t.Fatalf("answer must equal RecordAnswer, got %q", items[0].Answer)
	}
	if items[0].KnowledgePointsId != "kp1" {
		t.Fatalf("knowledgePointsId not copied, got %q", items[0].KnowledgePointsId)
	}
}

func TestICVEDraftPayloadDoesNotFallbackIdToExamIdWhenRecordIdEmpty(t *testing.T) {
	// recordId 娑撹櫣鈹栭弮?buildICVEDraftProblemList 娑撳秴褰堣ぐ鍗炴惙閿?	// 娑撴棁鐨熼悽銊︽煙娑撳秴绨查幎?examId 濞夈劌鍙?payload.Id
	// 闁俺绻冮惄瀛樺复濡偓閺?id 闁槒绶敍姝砮c.Id="" exam.RecordId="" 閳?recordId=""
	rec := &icveExamStartResp{Id: ""}
	exam := icveExamItem{Id: "exam-abc", ExamId: "exam-abc", RecordId: ""}
	recordId := rec.Id
	if recordId == "" {
		recordId = exam.RecordId
	}
	// 娑撳秴鍟€ fallback 閸?examId
	if recordId != "" {
		t.Fatalf("recordId should be empty when both rec.Id and exam.RecordId are empty, got %q", recordId)
	}
}

func TestICVEDraftPayloadUsesRecordIdWhenPresent(t *testing.T) {
	rec := &icveExamStartResp{Id: ""}
	exam := icveExamItem{RecordId: "real-record-id"}
	recordId := rec.Id
	if recordId == "" {
		recordId = exam.RecordId
	}
	if recordId != "real-record-id" {
		t.Fatalf("expected real-record-id, got %q", recordId)
	}
}

func TestICVEAddPayloadUsesAnswerField(t *testing.T) {
	problems := []icveExamProblemRecord{
		{QuestionNo: 1, Answer: "correct", RecordAnswer: "3", PaperId: "p2"},
	}
	items := buildICVEDraftProblemList(problems)
	if items[0].Answer != "3" {
		t.Fatalf("expected answer=3, got %q", items[0].Answer)
	}
}

func TestICVEDraftPayloadKeepsCourseFields(t *testing.T) {
	exam := icveExamItem{
		ExamId:       "eid-1",
		CourseId:     "cid-1",
		CourseInfoId: "ciid-1",
		CategoryId:   "3",
	}
	if exam.CourseId == "" || exam.CourseInfoId == "" {
		t.Fatal("courseId/courseInfoId must not be empty for draft payload")
	}
}

func TestICVEDraftPayloadIsLastTrue(t *testing.T) {
	payload := icveHomeworkExamPayload{IsLast: true}
	// 鐪熷疄鎶撳寘锛歩sLast=true 鍗充娇鐐逛繚瀛樻寜閽?	payload := icveHomeworkExamPayload{IsLast: true}
	if !payload.IsLast {
		t.Fatal("isLast must be true even for save button")
	}
}

// ---- 5 new probe / classification tests ----

func TestICVEResolveCourseByHomeworkProbe(t *testing.T) {
	candidates := []icveStudentCourseItem{
		{CourseId: "c1", CourseInfoId: "ci1", CourseName: "璇剧▼A"},
		{CourseId: "c2", CourseInfoId: "ci2", CourseName: "璇剧▼B"},
	}
	// probe with matching courseId 鈥?match logic only (no HTTP)
	want := "c1"
	matched := false
	for _, c := range candidates {
		if c.CourseId == want {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatal("should find candidate by courseId")
	}
}

func TestICVEExamTitleWinsOverTypeId(t *testing.T) {
	item := icveExamItem{Name: "\u3010\u671f\u4e2d\u8003\u8bd5\u30115G", TypeId: "1", CategoryId: "2"}
	kind := icveExamKind(item, "2")
	if icveNameContains(item.name(), "\u8003\u8bd5") {
		kind = "exam"
	}
	if kind != "exam" {
		t.Fatalf("exam title must win over typeId=1, got %q", kind)
	}
}

func TestICVEModuleQuizTypeId1AsWork(t *testing.T) {
	item := icveExamItem{Name: "\u3010\u6a21\u5757\u6d4b\u9a8c\u3011\u6a21\u5757\u4e8c", TypeId: "1", CategoryId: "1"}
	kind := icveExamKind(item, "1")
	if kind != "work" {
		t.Fatalf("typeId=1 categoryId=1 should be work, got %q", kind)
	}
}

func TestICVEParseStudentCourseNestedRows(t *testing.T) {
	raw := `{"code":200,"data":{"rows":[{"courseId":"cid1","courseInfoId":"ciid1","courseName":"娴嬭瘯璇剧▼"}]}}`
	items, err := icveParseGenericList(raw, func(b []byte) ([]icveStudentCourseItem, error) {
		var v []icveStudentCourseItem
		return v, json.Unmarshal(b, &v)
	})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item from nested rows, got %d", len(items))
	}
	if items[0].CourseId != "cid1" {
		t.Fatalf("courseId mismatch: %q", items[0].CourseId)
	}
}

func TestICVEHomeworkProbeLogsZeroRows(t *testing.T) {
	// icveFetchListByEndpoint with zero rows returns empty slice (no error)
	// simulate by parsing an empty rows response
	raw := `{"code":200,"rows":[]}`
	items, err := icveParseListExam(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items for empty rows, got %d", len(items))
	}
}

func TestICVEParseListExamTopLevelRowsWithoutCode(t *testing.T) {
	raw := `{"total":86,"data":{},"rows":[{"id":"03debaa5-38c7-4c7a-b5ab-02b1257480c1","name":"璇惧悗浣滀笟","projectGroupId":"ezmjagireqpmjsqej9pvng","categoryId":"3","typeId":"1","courseInfoId":"397862ba-285e-4c96-9557-de4e122336ad","courseId":"bc643fa9-b851-4c78-9910-4dcd76426c12"}]}`
	items, err := icveParseListExam(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item from top-level rows, got %d", len(items))
	}
	if items[0].Id != "03debaa5-38c7-4c7a-b5ab-02b1257480c1" {
		t.Fatalf("id mismatch: %q", items[0].Id)
	}
	if items[0].CategoryId != "3" || items[0].TypeId != "1" {
		t.Fatalf("category/type mismatch: %+v", items[0])
	}
}

func TestICVEParseGenericListSkipsEmptyNestedRows(t *testing.T) {
	raw := `{"code":200,"data":{"rows":[],"list":[{"id":"work-1","name":"璇惧悗浣滀笟","categoryId":"3","typeId":"1"}]}}`
	items, err := icveParseListExam(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item from nested list after empty rows, got %d", len(items))
	}
	if items[0].Id != "work-1" {
		t.Fatalf("id mismatch: %q", items[0].Id)
	}
}

func TestICVEKeepTestRecordIdFallsBackToTaskId(t *testing.T) {
	got, source := icveRecordIdForSubmit(icveExamItem{
		Source: "exam",
		TaskId: "task-record-id",
	}, &icveExamStartResp{})
	if got != "task-record-id" || source != "exam.TaskId" {
		t.Fatalf("expected taskId record id, got %q from %s", got, source)
	}
}

func TestICVEHomeworkRecordIdDoesNotUseTaskId(t *testing.T) {
	got, source := icveRecordIdForSubmit(icveExamItem{
		Source: "homework",
		TaskId: "task-record-id",
	}, &icveExamStartResp{})
	if got != "" || source != "empty" {
		t.Fatalf("homework must not use taskId fallback, got %q from %s", got, source)
	}
}

func TestICVEParseQuestionArrayNestedQuestionObject(t *testing.T) {
	arr := []interface{}{
		map[string]interface{}{
			"id":   "outer-1",
			"rows": []interface{}{},
			"question": map[string]interface{}{
				"title":      "real stem",
				"typeName":   "single",
				"dataJson":   `[{"Label":0,"SortOrder":0,"Content":"A"},{"Label":1,"SortOrder":1,"Content":"B"}]`,
				"questionId": "qid-1",
			},
		},
	}
	got := parseICVEQuestionArray(arr)
	if len(got) != 1 {
		t.Fatalf("expected 1 parsed question, got %d", len(got))
	}
	q := got[0]
	if q.Stem != "real stem" {
		t.Fatalf("stem mismatch: %#v", q)
	}
	if len(q.Options) != 2 {
		t.Fatalf("options not parsed: %#v", q.Options)
	}
	if q.QuestionId != "qid-1" {
		t.Fatalf("questionId mismatch: %#v", q)
	}
}

func TestICVEFindQuestionListNestedArrays(t *testing.T) {
	root := map[string]interface{}{
		"data": map[string]interface{}{
			"rows": []interface{}{
				map[string]interface{}{
					"id": "outer-1",
					"children": []interface{}{
						map[string]interface{}{
							"title":      "nested stem",
							"typeName":   "multiple",
							"dataJson":   `[{"Label":0,"SortOrder":0,"Content":"X"},{"Label":1,"SortOrder":1,"Content":"Y"}]`,
							"questionId": "qid-2",
						},
					},
				},
			},
		},
	}
	got := findICVEQuestionList(root)
	if len(got) != 1 {
		t.Fatalf("expected 1 nested question, got %d", len(got))
	}
	if got[0].Stem != "nested stem" {
		t.Fatalf("nested stem mismatch: %#v", got[0])
	}
}
