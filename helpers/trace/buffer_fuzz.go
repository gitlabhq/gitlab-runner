//go:build gofuzz

package trace

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func Fuzz(data []byte) int {
	buffer, err := New()
	if err != nil {
		return -1
	}
	defer buffer.Close()

	masks := common.MaskOptions{
		Phrases: []string{
			strings.Repeat("A", 1024),
			strings.Repeat("B", 4*1024),
			strings.Repeat("C", 8*1024),
			"secret",
			"secret_suffix",
			"ssecret",
			"secrett",
			"ssecrett",
		},
		TokenPrefixes: []string{
			"secret_prefix",
			"secret-prefix",
			"secret_prefix-",
			"secret-prefix-",
			"secret_prefix_",
			"secret-prefix_",
		},
	}
	// to be combined with TokenPrefixes
	secretSuffixes := []string{
		"THIS_IS_SECRET",
		"ALSO-SECRET",
	}

	buffer.SetMasked(masks)

	seed := data
	if len(seed) < 8 {
		seed = append(seed, make([]byte, 8-len(seed))...)
	}
	r := rand.New(rand.NewSource(int64(binary.BigEndian.Uint64(seed))))

	// copy fuzz input to new slice, with interspersed mask values at random locations
	var src []byte
	chunk(r, data, func(part []byte) {
		src = append(src, part...)
		if r.Intn(2) == 1 {
			src = append(src, []byte(masks.Phrases[r.Intn(len(masks.Phrases))])...)
		}
		if r.Intn(2) == 1 {
			pref := masks.TokenPrefixes[r.Intn(len(masks.TokenPrefixes))]
			suf := secretSuffixes[r.Intn(len(secretSuffixes))]
			src = append(src, []byte(pref+suf)...)
		}
	})

	// write src to buffer, but with random sized slices
	chunk(r, src, func(part []byte) {
		n, err := buffer.Write(part)
		if err != nil {
			panic(err)
		}
		if n != len(part) {
			panic(fmt.Sprintf("n(%d) < len(part)(%d)", n, len(part)))
		}
	})

	contents, err := buffer.Bytes(0, math.MaxInt64)
	if err != nil {
		panic(err)
	}

	for _, mask := range masks.Phrases {
		if bytes.Contains(contents, []byte(mask)) {
			panic(fmt.Sprintf("mask %q present in %q", mask, contents))
		}
	}

	for _, mask := range secretSuffixes {
		if bytes.Contains(contents, []byte(mask)) {
			panic(fmt.Sprintf("prefix mask %q present in %q", mask, contents))
		}
	}

	return 0
}

func chunk(r *rand.Rand, input []byte, fn func(part []byte)) {
	for {
		if len(input) == 0 {
			break
		}

		offset := 1 + r.Intn(len(input))
		fn(input[:offset])
		input = input[offset:]
	}
}
