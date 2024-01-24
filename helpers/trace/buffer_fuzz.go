//go:build gofuzz

package trace

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger/internal/masker"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger/internal/tokensanitizer"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger/internal/urlsanitizer"
)

type nopWriter struct {
	io.Writer
}

func (nopWriter) Close() error {
	return nil
}

func Fuzz(data []byte) int {
	phrases := []string{
		strings.Repeat("A", 1024),
		strings.Repeat("B", 4*1024),
		strings.Repeat("C", 8*1024),
		"secret",
		"secret_suffix",
		"ssecret",
		"secrett",
		"ssecrett",
	}

	tokenPrefixes := []string{
		"secret_prefix",
		"secret-prefix",
		"secret_prefix-",
		"secret-prefix-",
		"secret_prefix_",
		"secret-prefix_",
	}

	// to be combined with tokenPrefixes
	secretSuffixes := []string{
		"THIS_IS_SECRET",
		"ALSO-SECRET",
	}

	buf := new(bytes.Buffer)

	w := io.WriteCloser(nopWriter{buf})
	w = masker.New(w, phrases)
	w = tokensanitizer.New(w, tokenPrefixes)
	w = urlsanitizer.New(w)

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
			src = append(src, []byte(phrases[r.Intn(len(phrases))])...)
		}
		if r.Intn(2) == 1 {
			pref := tokenPrefixes[r.Intn(len(tokenPrefixes))]
			suf := secretSuffixes[r.Intn(len(secretSuffixes))]
			src = append(src, []byte(pref+suf)...)
		}
	})

	// write src to buffer, but with random sized slices
	chunk(r, src, func(part []byte) {
		n, err := w.Write(part)
		if err != nil {
			panic(err)
		}
		if n != len(part) {
			panic(fmt.Sprintf("n(%d) < len(part)(%d)", n, len(part)))
		}
	})

	contents := buf.Bytes()
	for _, mask := range phrases {
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
