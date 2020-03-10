// +build !solution

package spacecollapse

import (
	"regexp"
)

func CollapseSpaces(input string) string {
	re := regexp.MustCompile(`[\s\t\r\n]+`)
	return re.ReplaceAllString(input, " ")
}
