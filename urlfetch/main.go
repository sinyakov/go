// +build !solution

package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

func main() {
	argsWithoutProg := os.Args[1:]
	for _, url := range argsWithoutProg {
		resp, err := http.Get(url)
		if err != nil {
			os.Exit(1)
		}
		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			os.Exit(1)
		}
		fmt.Printf("%s", body)
	}
}
