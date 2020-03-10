// +build !solution

package reverse

import (
	"strings"
)

func Reverse(input string) string {
	runes := []rune(input)

	var sb strings.Builder

	for i := len(runes) - 1; i >= 0; i-- {
		sb.WriteRune(runes[i])
	}
	return sb.String()
}
