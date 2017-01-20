package spider

import (
	"strconv"
)

func FindMaxInt(min int, strs []string) int {
	max := min
	for _, v := range strs {
		n, _ := strconv.Atoi(v)
		if n > max {
			max = n
		}
	}
	return max
}
