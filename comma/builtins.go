package comma

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/midbel/slices"
)

type builtinFunc func([]string) (string, error)

var builtins = map[string]builtinFunc{
	// time functions
	"now":  checkArgs(0, false, runNow),
	"time": checkArgs(0, false, runTime),
	// string functions
	"trim":       checkArgs(0, false, runTrim),
	"lower":      checkArgs(0, false, runLower),
	"upper":      checkArgs(0, false, runUpper),
	"title":      checkArgs(0, false, runTitle),
	"replace":    checkArgs(0, false, runReplace),
	"join":       checkArgs(0, false, runJoin),
	"startswith": checkArgs(0, false, runStartsWith),
	"endswith":   checkArgs(0, false, runEndsWith),
	"contains":   checkArgs(0, false, runContains),
	// math functions
	"abs":  checkArgs(0, false, runAbs),
	"add":  checkArgs(0, false, runAdd),
	"mul":  checkArgs(0, false, runMul),
	"sub":  checkArgs(0, false, runSub),
	"div":  checkArgs(0, false, runDiv),
	"sqrt": checkArgs(0, false, runSqrt),
	"avg":  checkArgs(0, false, runAvg),
	"min":  checkArgs(0, false, runMin),
	"max":  checkArgs(0, false, runMax),
	// misc function
	"len":   checkArgs(0, false, runLen),
	"true":  checkArgs(0, false, runTrue),
	"false": checkArgs(0, false, runFalse),
	"if":    checkArgs(0, false, runIf),
	"any":   checkArgs(0, false, runIf),
	"all":   checkArgs(0, false, runAll),
}

func runNow(args []string) (string, error) {
	return time.Now().Format(time.RFC3339), nil
}

func runTime(args []string) (string, error) {
	n := time.Now()
	return strconv.FormatInt(n.Unix(), 10), nil
}

func runTrue(args []string) (string, error) {
	return "true", nil
}

func runFalse(args []string) (string, error) {
	return "false", nil
}

func runIf(args []string) (string, error) {
	if isTrue(slices.Fst(args)) {
		return slices.Snd(args), nil
	}
	return slices.Lst(args), nil
}

func runAll(args []string) (string, error) {
	if len(args) == 0 {
		return "false", nil
	}
	for i := range args {
		if !isTrue(args[i]) {
			return "false", nil
		}
	}
	return "true", nil
}

func runAny(args []string) (string, error) {
	if len(args) == 0 {
		return "false", nil
	}
	for i := range args {
		if isTrue(args[i]) {
			return "true", nil
		}
	}
	return "false", nil
}

func runLen(args []string) (string, error) {
	n := len(slices.Fst(args))
	return strconv.Itoa(n), nil
}

func runAbs(args []string) (string, error) {
	v, err := strconv.ParseFloat(slices.Fst(args), 64)
	if err != nil {
		return "", err
	}
	return strconv.FormatFloat(math.Abs(v), 'f', -1, 64), nil
}

func runMin(args []string) (string, error) {
	var res float64
	for _, str := range args {
		v, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return "", err
		}
		res = math.Min(res, v)
	}
	return strconv.FormatFloat(res, 'f', -1, 64), nil
}

func runMax(args []string) (string, error) {
	var res float64
	for _, str := range args {
		v, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return "", err
		}
		res = math.Max(res, v)
	}
	return strconv.FormatFloat(res, 'f', -1, 64), nil
}

func runSqrt(args []string) (string, error) {
	v, err := strconv.ParseFloat(slices.Fst(args), 64)
	if err != nil {
		return "", err
	}
	return strconv.FormatFloat(math.Sqrt(v), 'f', -1, 64), nil
}

func runAvg(args []string) (string, error) {
	n := len(args)
	if n == 0 {
		return "0", nil
	}
	var res float64
	for i := range args {
		v, err := strconv.ParseFloat(args[i], 64)
		if err != nil {
			return "", err
		}
		res += v
	}
	return strconv.FormatFloat(res/float64(n), 'f', -1, 64), nil
}

func runAdd(args []string) (string, error) {
	var res float64
	for i := range args {
		v, err := strconv.ParseFloat(args[i], 64)
		if err != nil {
			return "", err
		}
		res += v
	}
	return strconv.FormatFloat(res, 'f', -1, 64), nil
}

func runSub(args []string) (string, error) {
	var res float64
	for i := range args {
		v, err := strconv.ParseFloat(args[i], 64)
		if err != nil {
			return "", err
		}
		res -= v
	}
	return strconv.FormatFloat(res, 'f', -1, 64), nil
}

func runDiv(args []string) (string, error) {
	var res float64
	for i := range args {
		v, err := strconv.ParseFloat(args[i], 64)
		if err != nil {
			return "", err
		}
		if v == 0 {
			return "", ErrZero
		}
		res /= v
	}
	return strconv.FormatFloat(res, 'f', -1, 64), nil
}

func runMul(args []string) (string, error) {
	var res float64
	for i := range args {
		v, err := strconv.ParseFloat(args[i], 64)
		if err != nil {
			return "", err
		}
		res *= v
	}
	return strconv.FormatFloat(res, 'f', -1, 64), nil
}

func runTrim(args []string) (string, error) {
	return strings.TrimSpace(slices.Fst(args)), nil
}

func runLower(args []string) (string, error) {
	return strings.ToLower(slices.Fst(args)), nil
}

func runUpper(args []string) (string, error) {
	return strings.ToUpper(slices.Fst(args)), nil
}

func runTitle(args []string) (string, error) {
	return strings.ToTitle(slices.Fst(args)), nil
}

func runReplace(args []string) (string, error) {
	str := strings.ReplaceAll(slices.Fst(args), slices.Snd(args), slices.Lst(args))
	return str, nil
}

func runContains(args []string) (string, error) {
	ok := strings.Contains(slices.Fst(args), slices.Lst(args))
	return strconv.FormatBool(ok), nil
}

func runStartsWith(args []string) (string, error) {
	ok := strings.HasPrefix(slices.Fst(args), slices.Lst(args))
	return strconv.FormatBool(ok), nil
}

func runEndsWith(args []string) (string, error) {
	ok := strings.HasSuffix(slices.Fst(args), slices.Lst(args))
	return strconv.FormatBool(ok), nil
}

func runJoin(args []string) (string, error) {
	return strings.Join(slices.Slice(args), slices.Lst(args)), nil
}

func checkArgs(n int, variadic bool, do builtinFunc) builtinFunc {
	return func(args []string) (string, error) {
		return do(args)
	}
}

func isTrue(str string) bool {
	if str == "" {
		return false
	}
	if ok, err := strconv.ParseBool(str); err == nil {
		return ok
	}
	if n, err := strconv.ParseFloat(str, 64); err == nil {
		if n == 0 {
			return false
		}
		return true
	}
	return true
}
