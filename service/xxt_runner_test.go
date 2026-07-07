package service

import "testing"

func TestXXTExtractExamJumpURL(t *testing.T) {
	html := `<input type="hidden" id="examJumpUrl" value="/exam-ans/exam/phone/task-exam?courseId=1&amp;code=t8918554" />`
	got := xxtExtractExamJumpURL(html)
	want := `/exam-ans/exam/phone/task-exam?courseId=1&code=t8918554`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestXXTExtractExamCodeDoesNotTreatTypeAsCode(t *testing.T) {
	raw := `https://mooc1-api.chaoxing.com/exam-ans/android/mtaskmsgspecial?taskrefId=10808239&type=exam&code=`
	if got, src := xxtExtractExamCode(raw); got != "" || src != "" {
		t.Fatalf("unexpected code %q src %q", got, src)
	}
}

func TestXXTExtractExamCodeFromExplicitCode(t *testing.T) {
	raw := `window.examConfig = { examCode: "t8918554" }`
	got, src := xxtExtractExamCode(raw)
	if got != "t8918554" || src == "" {
		t.Fatalf("got %q src %q", got, src)
	}
}

func TestXXTExtractExamQuestionTotal(t *testing.T) {
	cases := []struct {
		html string
		want int
	}{
		{`<div aria-label='当前第1 题; 共6题'><span>1/6</span></div>`, 6},
		{`<h3>单选题（共6题，30.0分）</h3>`, 6},
		{`<span>1/12</span>`, 12},
	}
	for _, tc := range cases {
		if got := xxtExtractExamQuestionTotal(tc.html); got != tc.want {
			t.Fatalf("got %d want %d for %q", got, tc.want, tc.html)
		}
	}
}
