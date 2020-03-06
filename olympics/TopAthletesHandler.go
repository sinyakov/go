package main

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
)

func TopAthletesHandler(w http.ResponseWriter, r *http.Request) {
	var sportParam string
	var limitParam int
	var err error
	queries := r.URL.Query()

	if sportSlc, ok := queries["sport"]; ok {
		if sportSlc[0] != "" {
			sportParam = sportSlc[0]
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
		return athlete.Sport == sportParam
	}
	filtered := Filter(athletes, test)

	if len(filtered) == 0 {
		http.Error(w, "sport not found", http.StatusNotFound)
		return
	}

	filteredMap := GetAllAthlets(filtered)

	values := make([]*AthleteInfo, 0, len(filteredMap))

	for _, v := range filteredMap {
		values = append(values, v)
	}

	sort.Slice(values, func(i, j int) bool {
		if values[i].Medals.Gold != values[j].Medals.Gold {
			return values[i].Medals.Gold > values[j].Medals.Gold
		}
		if values[i].Medals.Silver != values[j].Medals.Silver {
			return values[i].Medals.Silver > values[j].Medals.Silver
		}
		if values[i].Medals.Bronze != values[j].Medals.Bronze {
			return values[i].Medals.Bronze > values[j].Medals.Bronze
		}

		return values[i].Athlete < values[j].Athlete
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
	_, err = w.Write(b)

	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
	}
}
