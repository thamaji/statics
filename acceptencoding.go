package statics

import (
	"sort"
	"strconv"
	"strings"
)

type AcceptEncoding struct {
	Algorithm    string  // gzip, compress, deflate, br, identity, *
	QualityValue float64 // 0～1, default=1
}

func ParseAcceptEncoding(values ...string) []AcceptEncoding {
	result := []AcceptEncoding{}
	for _, v := range values {
		for _, w := range strings.Split(v, ",") {
			if i := strings.LastIndex(w, ";q="); i > 0 {
				q, err := strconv.ParseFloat(strings.TrimSpace(w[i+3:]), 64)
				if err != nil {
					continue
				}
				result = append(result, AcceptEncoding{
					Algorithm:    strings.TrimSpace(w[:i]),
					QualityValue: q,
				})
			} else {
				result = append(result, AcceptEncoding{
					Algorithm:    strings.TrimSpace(w),
					QualityValue: 1,
				})
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].QualityValue > result[j].QualityValue
	})
	return result
}
