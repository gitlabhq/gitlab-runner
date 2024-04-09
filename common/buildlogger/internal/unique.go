package internal

import (
	"cmp"
	"slices"
	"strings"
)

func Unique(tokens []string) [][]byte {
	for idx, token := range tokens {
		tokens[idx] = strings.TrimSpace(token)
	}

	slices.SortFunc(tokens, func(a, b string) int {
		switch {
		case len(a) < len(b):
			return -1
		case len(a) > len(b):
			return 1
		}

		return cmp.Compare(a, b)
	})

	compact := slices.Compact(tokens)
	unique := make([][]byte, 0, len(compact))
	for _, token := range compact {
		if token == "" {
			continue
		}
		unique = append(unique, []byte(token))
	}

	return unique
}
