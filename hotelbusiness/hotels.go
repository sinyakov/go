// +build !solution

package hotelbusiness

import (
	"fmt"
	"sort"
)

type Guest struct {
	CheckInDate  int
	CheckOutDate int
}

type Load struct {
	StartDate  int
	GuestCount int
}

type Change struct {
	num  int
	sign int
}

func ComputeLoad(guests []Guest) []Load {
	var s []*Change
	var res []Load

	if len(guests) == 0 {
		return []Load{}
	}

	fmt.Println(len(guests))
	for i := 0; i < len(guests); i++ {
		s = append(s, &Change{guests[i].CheckInDate, 1})
		s = append(s, &Change{guests[i].CheckOutDate, -1})
	}

	sort.Slice(s, func(i, j int) bool {
		return s[i].num < s[j].num
	})

	var current int = s[0].sign
	var day int = s[0].num

	for i := 1; i < len(s); i++ {
		if s[i].num != s[i-1].num {
			if len(res) == 0 || res[len(res)-1].GuestCount != current {
				res = append(res, Load{day, current})
			}
		}
		current += s[i].sign
		day = s[i].num
	}

	res = append(res, Load{day, current})

	return res
}
