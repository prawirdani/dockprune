package main

import (
	"testing"
	"time"
)

func TestRelTime(t *testing.T) {
	now := time.Now().Unix()
	cases := []struct {
		sec  int64
		want string
	}{
		{0, "unknown"},
		{now - 30, "just now"},
		{now - 5*60, "5m ago"},
		{now - 3*3600, "3h ago"},
		{now - 50*3600, "2d ago"},
	}
	for _, c := range cases {
		if got := relTime(c.sec); got != c.want {
			t.Errorf("relTime(%d)=%q want %q", c.sec, got, c.want)
		}
	}
}

func TestHumanSize(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "-"},
		{-5, "-"},
		{512, "512B"},
		{1500, "1.5kB"},
		{2_000_000, "2.0MB"},
		{3_000_000_000, "3.0GB"},
	}
	for _, c := range cases {
		if got := humanSize(c.in); got != c.want {
			t.Errorf("humanSize(%d)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in, want string
		max      int
	}{
		{"hello", "hello", 10},
		{"hello world", "hell…", 5},
		{"héllo wörld", "héll…", 5},
		{"x", "", 0},
	}
	for _, c := range cases {
		if got := truncate(c.in, c.max); got != c.want {
			t.Errorf("truncate(%q,%d)=%q want %q", c.in, c.max, got, c.want)
		}
	}
}

func TestOneLine(t *testing.T) {
	cases := []struct{ in, want string }{
		{"single", "single"},
		{"first\nsecond", "first"},
		{"a\nb\nc", "a"},
		{"", ""},
	}
	for _, c := range cases {
		if got := oneLine(c.in); got != c.want {
			t.Errorf("oneLine(%q)=%q want %q", c.in, got, c.want)
		}
	}
}
