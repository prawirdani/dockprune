package main

import "testing"

func TestFirstName(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"/web"}, "web"},
		{[]string{"web"}, "web"},
		{[]string{"/a", "/b"}, "a"},
	}
	for _, c := range cases {
		if got := firstName(c.in); got != c.want {
			t.Errorf("firstName(%v)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestParseRFC3339(t *testing.T) {
	if got := parseRFC3339(""); got != 0 {
		t.Errorf("empty => %d want 0", got)
	}
	if got := parseRFC3339("not-a-time"); got != 0 {
		t.Errorf("bad => %d want 0", got)
	}
	if got := parseRFC3339("2021-01-02T03:04:05Z"); got == 0 {
		t.Error("valid RFC3339 should parse to non-zero unix seconds")
	}
}

func TestSortItems(t *testing.T) {
	items := []item{{name: "c"}, {name: "a"}, {name: "b"}}
	sortItems(items)
	for i, want := range []string{"a", "b", "c"} {
		if items[i].name != want {
			t.Fatalf("sorted[%d]=%q want %q", i, items[i].name, want)
		}
	}
}
