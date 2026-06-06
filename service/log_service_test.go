package service

import "testing"

func TestStripANSI(t *testing.T) {
	input := "\x1b[0;37m[\x1b[0m\x1b[0;32m18075982110\x1b[0m] \x1b[0;35m课程学习完毕\x1b[0m"
	want  := "[18075982110] 课程学习完毕"
	if got := StripANSI(input); got != want {
		t.Errorf("StripANSI()\ngot  %q\nwant %q", got, want)
	}
}

func TestPushStripsANSI(t *testing.T) {
	h := NewLogHub()
	h.Push("\x1b[0;32m课程学习完毕\x1b[0m")
	lines := h.Recent(1)
	if len(lines) != 1 || lines[0] != "课程学习完毕" {
		t.Errorf("Push did not strip ANSI, got: %q", lines)
	}
}
