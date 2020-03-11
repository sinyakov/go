// +build !solution

package hogwarts

func dfs(x string, used *map[string]int, prereqs *map[string][]string, res *[]string) {
	(*used)[x] = 1

	for _, y := range (*prereqs)[x] {
		if (*used)[y] == 0 {
			dfs(y, used, prereqs, res)
		} else if (*used)[y] == 1 {
			panic("cycle")
		}
	}

	(*used)[x] = 2

	*res = append(*res, x)
}

func GetCourseList(prereqs map[string][]string) (res []string) {
	used := make(map[string]int)

	for x := range prereqs {
		if used[x] == 0 {
			dfs(x, &used, &prereqs, &res)
		}
	}

	return res
}
