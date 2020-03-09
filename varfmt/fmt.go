// +build !solution

package varfmt

import (
	"fmt"
	"strconv"
	"strings"
)

func Sprintf(format string, args ...interface{}) string {
	var sb strings.Builder

	argIdx := 0
	isOpened := false
	var prevOpenIdx int
	var prevCloseIdx int = -1

	argsString := make([]string, len(args))

	for idx := range args {
		argsString[idx] = fmt.Sprintf("%v", args[idx])
	}

	for runeIdx := range format {
		if format[runeIdx] == '{' {
			sb.WriteString(format[prevCloseIdx+1 : runeIdx])
			isOpened = true
			prevOpenIdx = runeIdx
			continue
		}
		if format[runeIdx] == '}' && isOpened {
			prevCloseIdx = runeIdx
			if runeIdx == prevOpenIdx+1 {
				sb.WriteString(fmt.Sprintf("%v", args[argIdx]))
			} else {
				number, _ := strconv.Atoi(format[prevOpenIdx+1 : runeIdx])

				sb.WriteString(argsString[number])
			}
			argIdx++
			isOpened = false
			continue
		}
	}

	sb.WriteString(format[prevCloseIdx+1:])

	return sb.String()
}
