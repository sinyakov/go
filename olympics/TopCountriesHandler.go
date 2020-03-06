package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
)

func TopCountriesHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("========")
	var yearParam int
	var limitParam int
	var err error
	queries := r.URL.Query()

	if yearSlc, ok := queries["year"]; ok {
		if yearSlc[0] != "" {
			yearParam, err = strconv.Atoi(yearSlc[0])
			if err != nil {
				http.Error(w, "invalid year param", http.StatusBadRequest)
				return
			}
		} else {
			http.Error(w, "no sport param", http.StatusBadRequest)
		}
	}

	if limitSlc, ok := queries["limit"]; ok {
		if limitSlc[0] != "" {
			limitParam, err = strconv.Atoi(limitSlc[0])
			if err != nil {
				http.Error(w, "invalid limit param", http.StatusBadRequest)
				return
			}
		}
	} else {
		limitParam = 3
	}

	test := func(athlete Athlete) bool {
		return athlete.Year == yearParam
	}
	filtered := Filter(athletes, test)

	if len(filtered) == 0 {
		http.Error(w, "year not found", http.StatusNotFound)
		return
	}

	filteredMap := GetAllCountries(filtered)

	values := make([]*CountryInfo, 0, len(filteredMap))

	for _, v := range filteredMap {
		values = append(values, v)
	}

	sort.Slice(values, func(i, j int) bool {
		if values[i].Gold != values[j].Gold {
			return values[i].Gold > values[j].Gold
		}
		if values[i].Silver != values[j].Silver {
			return values[i].Silver > values[j].Silver
		}
		if values[i].Bronze != values[j].Bronze {
			return values[i].Bronze > values[j].Bronze
		}

		return values[i].Country < values[j].Country
	})

	limit := int(math.Min(float64(limitParam), float64(len(values))))
	result := values[:limit]
	b, err := json.Marshal(&result)

	if err != nil {
		http.Error(w, "server error", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}
