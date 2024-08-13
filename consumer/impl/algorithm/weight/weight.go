package weight

import (
	"math/rand"

	"github.com/samber/lo"
)

func GetEndpoint(weightSlice []int64) int {
	sum := lo.Sum(weightSlice)
	randNum := rand.Int63n(sum)

	res := int64(0)
	for i, weight := range weightSlice {
		res += weight
		if randNum < res {
			return i
		}
	}
	return -1
}
