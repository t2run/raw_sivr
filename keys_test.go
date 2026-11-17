package main

import "testing"

func TestNormalizeKeys(t *testing.T) {
	keys, err := normalizeKeys(map[string]string{"pick": "A", "bogus": "q"})
	if err != nil {
		t.Fatal(err)
	}
	if keys["pick"] != "a" || keys["reject"] != "x" || len(keys) != len(defaultKeys) {
		t.Errorf("keys = %v", keys)
	}
	for _, bad := range []map[string]string{
		{"pick": "x"},  // collides with the default for reject
		{"pick": "ab"}, // more than one character
		{"zoom": "3"},  // reserved for compare panes
		{"zoom": " "},  // reserved for play / open
	} {
		if _, err := normalizeKeys(bad); err == nil {
			t.Errorf("%v should be rejected", bad)
		}
	}
}
