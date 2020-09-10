// +build gofuzz

package trace

import (
	"fmt"
	"log"
	"strings"
)

func Fuzz(data []byte) int {
	maskedValues := []string{
		"is",
		"duplicateValue",
		"duplicateValue",
		":secret",
		"cont@ining",
	}
	origstr := string(data)

	buffer, err := New()
	if err != nil {
		log.Fatal(err)
	}
	defer buffer.Close()

	buffer.SetMasked(maskedValues)

	_, err = buffer.Write(data)
	if err != nil {
		log.Fatal(err)
	}

	content, err := buffer.Bytes(0, 1000)
	if err != nil {
		log.Fatal(err)
	}

	newstr := string(content)
	for _, substr := range maskedValues {
		if strings.Contains(newstr, substr) {
			panic(fmt.Sprintf("orig string: %q, new string: %q contains: %q", origstr, newstr, substr))

		}
	}

	return 0
}
