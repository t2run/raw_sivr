package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// defaultKeys maps each action that can be rebound to its key. Arrows,
// Enter, Escape, Space and 2/3/4 are fixed.
var defaultKeys = map[string]string{
	"pick":     "p",
	"reject":   "x",
	"clear":    "u",
	"zoom":     "z",
	"info":     "i",
	"compare":  "c",
	"burst":    "b",
	"keepBest": "k",
	"original": "o",
}

// normalizeKeys fills in defaults for actions that are missing and rejects
// keys that are not a single character, are reserved, or are used twice.
func normalizeKeys(keys map[string]string) (map[string]string, error) {
	out := map[string]string{}
	used := map[string]string{}
	for action, def := range defaultKeys {
		k := strings.ToLower(keys[action])
		if k == "" {
			k = def
		}
		if utf8.RuneCountInString(k) != 1 || strings.ContainsAny(k, " 234") {
			return nil, fmt.Errorf("%q cannot be used as a shortcut (one character; space, 2, 3 and 4 are taken)", keys[action])
		}
		if other, taken := used[k]; taken {
			return nil, fmt.Errorf("the key %q is used for both %s and %s", strings.ToUpper(k), other, action)
		}
		used[k] = action
		out[action] = k
	}
	return out, nil
}
