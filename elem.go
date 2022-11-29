package query

import (
	"strconv"
)

func getBool(str string) (bool, error) {
	return strconv.ParseBool(str)
}

func getFloat(str string) (float64, error) {
	return strconv.ParseFloat(str, 64)
}
