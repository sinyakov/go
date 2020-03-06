// +build !solution

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

type Athlete struct {
	Athlete string `json:"athlete"`
	Age     int    `json:"age"`
	Country string `json:"country"`
	Year    int    `json:"year"`
	Date    string `json:"date"`
	Sport   string `json:"sport"`
	Gold    int    `json:"gold"`
	Silver  int    `json:"silver"`
	Bronze  int    `json:"bronze"`
	Total   int    `json:"total"`
}

type Medals struct {
	Gold   int `json:"gold"`
	Silver int `json:"silver"`
	Bronze int `json:"bronze"`
	Total  int `json:"total"`
}

type AthleteInfo struct {
	Athlete      string          `json:"athlete"`
	Country      string          `json:"country"`
	Medals       Medals          `json:"medals"`
	MedalsByYear map[int]*Medals `json:"medals_by_year"`
}

type CountryInfo struct {
	Country string `json:"country"`
	Gold    int    `json:"gold"`
	Silver  int    `json:"silver"`
	Bronze  int    `json:"bronze"`
	Total   int    `json:"total"`
}

var athletes []Athlete

func main() {
	port := flag.String("port", "80", "http server port")
	jsonPath := flag.String("data", "./olympics/testdata/olympicWinners.json", "json path")
	flag.Parse()

	jsonFile, err := os.Open(*jsonPath)
	if err != nil {
		log.Fatal("read file error")
	}

	byteValue, _ := ioutil.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, &athletes)

	if err != nil {
		log.Fatal("json parsing error")
		return
	}

	defer jsonFile.Close()

	http.HandleFunc("/athlete-info", AthleteHandler)
	http.HandleFunc("/top-athletes-in-sport", TopAthletesHandler)
	http.HandleFunc("/top-countries-in-year", TopCountriesHandler)

	host := fmt.Sprintf(":%s", *port)
	log.Fatal(http.ListenAndServe(host, nil))
}
