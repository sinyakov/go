// +build !solution

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

func main() {
	filenames := os.Args[1:]
	counts := map[string]int{}

	for i := 0; i < len(filenames); i++ {
		data, _ := ioutil.ReadFile(filenames[i])
		lines := strings.Split(string(data), "\n")
		for j := 0; j < len(lines); j++ {
			counts[lines[j]]++
		}
	}

	for key, value := range counts {
		if value > 1 {
			fmt.Printf("%d\t%s\n", value, key)
		}
	}
}
