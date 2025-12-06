package utils

import (
	"math"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
)

func ReplaceQueryParams(namedQuery string, params map[string]interface{}) (string, []interface{}) {
	var (
		i    int = 1
		args []interface{}
	)

	for k, v := range params {
		if k != "" {
			namedQuery = strings.ReplaceAll(namedQuery, ":"+k, "$"+strconv.Itoa(i))

			args = append(args, v)
			i++
		}
	}

	return namedQuery, args
}

func FCurrency(n float64) string {
	if n == 0 {
		return ""
	}

	rounded := math.Round(n*100) / 100
	formatted := humanize.CommafWithDigits(rounded, 2)

	if !strings.Contains(formatted, ".") {
		formatted += " "
	}

	return formatted
}
