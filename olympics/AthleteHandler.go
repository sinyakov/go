package main

import (
	"encoding/json"
	"net/http"
)

func AthleteHandler(w http.ResponseWriter, r *http.Request) {
	var name string
	queries := r.URL.Query()

	if nameSlc, ok := queries["name"]; ok {
		if nameSlc[0] != "" {
			name = nameSlc[0]
		} else {
			http.Error(w, "no name param", http.StatusBadRequest)
		}
	}

	test := func(athlete Athlete) bool {
		return athlete.Athlete == name
	}

	filtered := Filter(athletes, test)

	if len(filtered) == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	medalsByYear := make(map[int]*Medals)
	var medalsTotal Medals

	for _, athlete := range filtered {
		_, ok := medalsByYear[athlete.Year]
		if !ok {
			medalsByYear[athlete.Year] = &Medals{0, 0, 0, 0}
		}
		medalsByYear[athlete.Year].Gold += athlete.Gold
		medalsByYear[athlete.Year].Silver += athlete.Silver
		medalsByYear[athlete.Year].Bronze += athlete.Bronze
		medalsByYear[athlete.Year].Total += athlete.Gold + athlete.Silver + athlete.Bronze

		medalsTotal.Gold += athlete.Gold
		medalsTotal.Silver += athlete.Silver
		medalsTotal.Bronze += athlete.Bronze
		medalsTotal.Total += athlete.Gold + athlete.Silver + athlete.Bronze
	}

	var resp AthleteInfo

	resp.Athlete = name
	resp.Country = filtered[0].Country
	resp.Medals = medalsTotal
	resp.MedalsByYear = medalsByYear
	b, err := json.Marshal(&resp)
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
