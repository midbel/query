package comma

import (
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/midbel/slices"
	"github.com/midbel/uuid"
)

type builtinFunc func([]string) (string, error)

var builtins = map[string]builtinFunc{
	// time functions
	"now":  checkArgs(0, true, runNow),
	"time": checkArgs(0, false, runTime),
	// string functions
	"trim":       checkArgs(1, false, runTrim),
	"lower":      checkArgs(1, false, runLower),
	"upper":      checkArgs(1, false, runUpper),
	"title":      checkArgs(1, false, runTitle),
	"replace":    checkArgs(3, false, runReplace),
	"join":       checkArgs(0, true, runJoin),
	"startswith": checkArgs(2, false, runStartsWith),
	"endswith":   checkArgs(2, false, runEndsWith),
	"contains":   checkArgs(2, false, runContains),
	// base64
	"b64encode": checkArgs(1, false, runEncodeB64),
	"b64decode": checkArgs(1, false, runDecodeB64),
	// math functions
	"abs":    checkArgs(1, false, runAbs),
	"add":    checkArgs(2, true, runAdd),
	"mul":    checkArgs(2, true, runMul),
	"sub":    checkArgs(2, true, runSub),
	"div":    checkArgs(2, true, runDiv),
	"avg":    checkArgs(2, true, runAvg),
	"sqrt":   checkArgs(1, false, runSqrt),
	"min":    checkArgs(2, true, runMin),
	"max":    checkArgs(2, true, runMax),
	"lshift": checkArgs(2, false, runShiftLeft),
	"rshift": checkArgs(2, false, runShiftRight),
	// misc function
	"len":   checkArgs(1, false, runLen),
	"true":  checkArgs(0, false, runTrue),
	"false": checkArgs(0, false, runFalse),
	"if":    checkArgs(3, false, runIf),
	"and":   checkArgs(2, false, runAnd),
	"or":    checkArgs(2, false, runOr),
	"any":   checkArgs(1, true, runIf),
	"all":   checkArgs(1, true, runAll),
	"uuid":  checkArgs(0, false, runUuid),
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

func runAnd(args []string) (string, error) {
	ok := isTrue(slices.Fst(args)) && isTrue(slices.Lst(args))
	return strconv.FormatBool(ok), nil
}

func runOr(args []string) (string, error) {
	ok := isTrue(slices.Fst(args)) || isTrue(slices.Lst(args))
	return strconv.FormatBool(ok), nil
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

func runUuid(args []string) (string, error) {
	uid := uuid.UUID4()
	return uid.String(), nil
}

func runShiftLeft(args []string) (string, error) {
	left, err := strconv.Atoi(slices.Fst(args))
	if err != nil {
		return "", castNumberError(slices.Fst(args))
	}
	right, err := strconv.Atoi(slices.Lst(args))
	if err != nil {
		return "", castNumberError(slices.Lst(args))
	}
	return strconv.Itoa(left << right), nil
}

func runShiftRight(args []string) (string, error) {
	left, err := strconv.Atoi(slices.Fst(args))
	if err != nil {
		return "", castNumberError(slices.Fst(args))
	}
	right, err := strconv.Atoi(slices.Lst(args))
	if err != nil {
		return "", castNumberError(slices.Lst(args))
	}
	return strconv.Itoa(left >> right), nil
}

func runAbs(args []string) (string, error) {
	v, err := strconv.ParseFloat(slices.Fst(args), 64)
	if err != nil {
		return "", castNumberError(slices.Fst(args))
	}
	return strconv.FormatFloat(math.Abs(v), 'f', -1, 64), nil
}

func runMin(args []string) (string, error) {
	var res float64
	for _, str := range args {
		v, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return "", castNumberError(str)
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
			return "", castNumberError(str)
		}
		res = math.Max(res, v)
	}
	return strconv.FormatFloat(res, 'f', -1, 64), nil
}

func runSqrt(args []string) (string, error) {
	v, err := strconv.ParseFloat(slices.Fst(args), 64)
	if err != nil {
		return "", castNumberError(slices.Fst(args))
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
			return "", castNumberError(args[i])
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
			return "", castNumberError(args[i])
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
			return "", castNumberError(args[i])
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
			return "", castNumberError(args[i])
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
			return "", castNumberError(args[i])
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

func runEncodeB64(args []string) (string, error) {
	in := slices.Fst(args)
	str := base64.StdEncoding.EncodeToString([]byte(in))
	return str, nil
}

func runDecodeB64(args []string) (string, error) {
	in := slices.Fst(args)
	str, err := base64.StdEncoding.DecodeString(in)
	return string(str), err
}

func checkArgs(n int, variadic bool, do builtinFunc) builtinFunc {
	return func(args []string) (string, error) {
		if x := len(args); x != n {
			if x < n {
				return "", ErrArgument
			}
			if !variadic {
				return "", ErrArgument
			}
		}
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

func castNumberError(str string) error {
	return fmt.Errorf("%w: %s can not be casted to number", ErrCast, str)
}
