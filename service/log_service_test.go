package service

import (
	"strings"
	"testing"
)

func TestStripANSI(t *testing.T) {
	input := "\x1b[0;37m[\x1b[0m\x1b[0;32m18075982110\x1b[0m] \x1b[0;35m\u8bfe\u7a0b\u5b66\u4e60\u5b8c\u6bd5\x1b[0m"
	want := "[18075982110] \u8bfe\u7a0b\u5b66\u4e60\u5b8c\u6bd5"
	if got := StripANSI(input); got != want {
		t.Errorf("StripANSI()\ngot  %q\nwant %q", got, want)
	}
}

func TestPushStripsANSIAndNormalizes(t *testing.T) {
	h := NewLogHub()
	h.Push("\x1b[0;32m\u8bfe\u7a0b\u5b66\u4e60\u5b8c\u6bd5\x1b[0m")
	lines := h.Recent(1)
	if len(lines) != 1 || !strings.Contains(lines[0], "\u8bfe\u7a0b") || strings.Contains(lines[0], "\\x1b") {
		t.Errorf("Push did not normalize ANSI log, got: %q", lines)
	}
}

func TestNormalizeLogTextFixesCommonMojibake(t *testing.T) {
	input := "[\u7f02\u4f7a\u7cba] worker \u9a9e\u8dfa\u5f42\uff1a\u5f53\u524d 1/3"
	got := NormalizeLogText(input)
	for _, want := range []string{"[\u7cfb\u7edf]", "\u5e76\u53d1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("NormalizeLogText()=%q, want contains %q", got, want)
		}
	}
}

func TestIsCoreLogAcceptsNormalizedChinese(t *testing.T) {
	if !isCoreLog("[\u7cfb\u7edf] \u8bfe\u7a0b [\u79fb\u52a8\u901a\u4fe1\u6280\u672f] \u83b7\u53d6\u5230\u9898\u76ee 10 \u9053") {
		t.Fatal("expected isCoreLog to accept normal Chinese log")
	}
}

func TestFixUTF8AsGBKMojibakeKeepsNormalChinese(t *testing.T) {
	inputs := []string{
		"[\u7cfb\u7edf] \u8bfe\u7a0b [\u79fb\u52a8\u901a\u4fe1\u6280\u672f] \u83b7\u53d6\u5230\u9898\u76ee 10 \u9053",
		"\u4fdd\u5b58\u7b54\u6848\u5931\u8d25: \u4e0d\u5728\u4f5c\u7b54\u65f6\u95f4\u5185",
		"\u5df2\u63d0\u4ea4\uff0c\u7b54\u6848\u5feb\u7167\uff1a\u98981=A | \u98982=B",
		"\u5b8c\u6210\u5ea6 80%\uff0c\u8fbe\u5230\u667a\u80fd\u63d0\u4ea4\u9608\u503c 60%\uff0c\u5df2\u63d0\u4ea4\uff0c8/10 \u5df2\u7b54",
	}
	for _, s := range inputs {
		got := fixUTF8AsGBKMojibake(s)
		if got != s {
			t.Errorf("fixUTF8AsGBKMojibake corrupted normal Chinese\ninput: %q\ngot:   %q", s, got)
		}
	}
}

func TestNormalizeLogTextKeepsNormalChinese(t *testing.T) {
	cases := []string{
		"[\u7cfb\u7edf] worker \u5e76\u53d1\uff1a\u5f53\u524d 1/3 | [ICVE] \u83b7\u53d6\u5230\u9898\u76ee 10 \u9053",
		"\u3010\u79fb\u52a8\u901a\u4fe1\u6280\u672f\u3011\u7b54\u6848\u5feb\u7167\uff1a\u98981=A | \u98982=B | \u98983=\u6b63\u786e",
		"\u5e73\u53f0\u63d0\u793a\u4e0d\u5728\u4f5c\u7b54\u65f6\u95f4\u5185",
	}
	for _, s := range cases {
		got := normalizeLogText(s)
		if got != s {
			t.Errorf("normalizeLogText corrupted normal Chinese\ninput: %q\ngot:   %q", s, got)
		}
	}
}
