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

type collector interface {
	collect(string, interface{})
	get() interface{}
}

type objCollector map[string]interface{}

func createObject() collector {
	return make(objCollector)
}

func (c objCollector) collect(k string, v interface{}) {
	if v == nil {
		return
	}
	c[k] = v
}

func (c objCollector) get() interface{} {
	return map[string]interface{}(c)
}

type arrCollector []interface{}

func createArray() collector {
	var arr arrCollector
	return &arr
}

func (c *arrCollector) collect(_ string, v interface{}) {
	if v == nil {
		return
	}
	*c = append(*c, v)
}

func (c *arrCollector) get() interface{} {
	return firstOrAll(*c)
}

func firstOrAll(arr []interface{}) interface{} {
	if len(arr) == 1 {
		return arr[0]
	}
	return arr
}
