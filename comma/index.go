package comma

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

var (
	ErrIndex   = errors.New("index out of range")
	ErrSupport = errors.New("unsupported operation")
	ErrZero    = errors.New("division by zero")
)

type Indexer interface {
	Index([]string) (string, error)
}

type ternary struct {
	cdt Indexer
	csq Indexer
	alt Indexer
}

func (t *ternary) Index(row []string) (string, error) {
	res, err := t.cdt.Index(row)
	if err != nil {
		return "", err
	}
	if isTrue(res) {
		return t.csq.Index(row)
	}
	return t.alt.Index(row)
}

type binary struct {
	left  Indexer
	right Indexer
	op    rune
}

func (b *binary) Index(row []string) (string, error) {
	left, err := b.left.Index(row)
	if err != nil {
		return "", err
	}
	right, err := b.right.Index(row)
	if err != nil {
		return "", err
	}
	return apply(left, right, func(left, right float64) (float64, error) {
		switch b.op {
		case Add:
			left += right
		case Sub:
			left -= right
		case Mul:
			left *= right
		case Pow:
			left = math.Pow(left, right)
		case Div:
			if right == 0 {
				return 0, ErrZero
			}
			left /= right
		case Mod:
			if right == 0 {
				return 0, ErrZero
			}
			left = math.Mod(left, right)
		default:
			return 0, ErrSupport
		}
		return left, nil
	})
}

type unary struct {
	right Indexer
	op    rune
}

func (u *unary) Index(row []string) (string, error) {
	got, err := u.right.Index(row)
	if err != nil {
		return "", err
	}
	n, err := strconv.ParseFloat(got, 64)
	if err != nil {
		return "", err
	}
	switch u.op {
	case Sub:
		return strconv.FormatFloat(-n, 'f', -1, 64), nil
	case Not:
		return strconv.FormatBool(n != 0), nil
	default:
		return "", ErrSupport
	}
}

type group struct {
	list []Indexer
}

func (g *group) Index(row []string) (string, error) {
	var str strings.Builder
	for i := range g.list {
		if i > 0 {
			str.WriteRune(',')
			str.WriteRune(' ')
		}

		got, err := g.list[i].Index(row)
		if err != nil {
			return "", err
		}
		str.WriteString(got)
	}
	return str.String(), nil
}

type object struct {
	fields map[string]Indexer
	keys   []string
}

func (o *object) Index(row []string) (string, error) {
	var str strings.Builder
	str.WriteRune('{')
	for i, k := range o.keys {
		if i > 0 {
			str.WriteRune(',')
			str.WriteRune(' ')
		}

		str.WriteString(withQuote(k, true))
		str.WriteRune(':')
		str.WriteRune(' ')

		val, err := o.fields[k].Index(row)
		if err != nil {
			return "", err
		}
		str.WriteString(val)
	}
	str.WriteRune('}')
	return str.String(), nil
}

type array struct {
	list []Indexer
}

func (a *array) Index(row []string) (string, error) {
	var str strings.Builder
	str.WriteRune('[')
	for i := range a.list {
		if i > 0 {
			str.WriteRune(',')
			str.WriteRune(' ')
		}
		got, err := a.list[i].Index(row)
		if err != nil {
			return "", err
		}
		str.WriteString(got)
	}
	str.WriteRune(']')
	return str.String(), nil
}

type set struct {
	index []Indexer
}

func (i *set) Index(row []string) (string, error) {
	var str strings.Builder
	str.WriteRune('[')
	for j := range i.index {
		if j > 0 {
			str.WriteRune(',')
			str.WriteRune(' ')
		}
		got, err := i.index[j].Index(row)
		if err != nil {
			return "", err
		}
		str.WriteString(got)
	}
	str.WriteRune(']')
	return str.String(), nil
}

type index struct {
	index int
}

func (i *index) Index(row []string) (string, error) {
	if i.index < 0 || i.index >= len(row) {
		return "", ErrIndex
	}
	return withQuote(row[i.index], false), nil
}

type interval struct {
	beg  int
	end  int
	add  bool
	flat bool
}

func (i *interval) Index(row []string) (string, error) {
	if i.end < i.beg {
		i.beg, i.end = i.end, i.beg
	}
	if i.beg < 0 || i.beg > len(row) {
		return "", ErrIndex
	}
	if i.end < 0 || i.end > len(row) {
		return "", ErrIndex
	}
	if !i.add {
		return i.asArray(row)
	}
	return i.asValue(row)
}

func (i *interval) asValue(row []string) (string, error) {
	var res float64
	for _, str := range row[i.beg : i.end+1] {
		v, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return "", err
		}
		res += v
	}
	return strconv.FormatFloat(res, 'f', -1, 64), nil
}

func (i *interval) asArray(row []string) (string, error) {
	var (
		str strings.Builder
		pos int
	)
	if !i.flat {
		str.WriteRune('[')
	}
	for j := i.beg; j <= i.end; j++ {
		if pos > 0 {
			str.WriteRune(',')
			str.WriteRune(' ')
		}
		pos++
		str.WriteString(withQuote(row[j], false))
	}
	if !i.flat {
		str.WriteRune(']')
	}
	return str.String(), nil
}

type literal struct {
	value string
}

func (i *literal) Index([]string) (string, error) {
	return withQuote(i.value, false), nil
}

func withQuote(str string, all bool) string {
	if str == "true" || str == "false" || str == "null" {
		return str
	}
	if str[0] == '"' && str[len(str)-1] == '"' {
		return str
	}
	if !all {
		_, err := strconv.ParseFloat(str, 64)
		if err == nil {
			return str
		}
	}
	return fmt.Sprintf("%q", str)
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

func apply(left, right string, do func(float64, float64) (float64, error)) (string, error) {
	x, err := strconv.ParseFloat(left, 64)
	if err != nil {
		return "", err
	}
	y, err := strconv.ParseFloat(right, 64)
	if err != nil {
		return "", err
	}
	res, err := do(x, y)
	if err != nil {
		return "", err
	}
	return strconv.FormatFloat(res, 'f', -1, 64), nil
}
