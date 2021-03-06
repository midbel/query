package query

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

var ErrEmpty = errors.New("empty queryset")

type ParseError struct {
	tok  Token
	ctx  string
	want string
}

func (e ParseError) Error() string {
	return fmt.Sprintf("%s: unexpected token %s", e.ctx, e.tok)
}

func parseError(where string, tok Token) error {
	return ParseError{
		tok: tok,
		ctx: where,
	}
}

type Parser struct {
	scan *Scanner
	curr Token
	peek Token

	selectors map[rune]func(rune) (Selector, error)
}

func Parse(str string) (Queryer, error) {
	p := NewParser(str)
	return p.Parse()
}

func NewParser(str string) *Parser {
	var p Parser
	p.scan = NewScanner(str)
	p.selectors = map[rune]func(rune) (Selector, error){
		TokSelectFirst:  p.parseSelectSimple,
		TokSelectLast:   p.parseSelectSimple,
		TokSelectInt:    p.parseSelectSimple,
		TokSelectFloat:  p.parseSelectSimple,
		TokSelectNumber: p.parseSelectSimple,
		TokSelectBool:   p.parseSelectSimple,
		TokSelectString: p.parseSelectSimple,
		TokSelectTruthy: p.parseSelectSimple,
		TokSelectFalsy:  p.parseSelectSimple,
		TokSelectAt:     p.parseSelectAt,
		TokSelectRange:  p.parseSelectRange,
	}
	p.next()
	p.next()

	return &p
}

func (p *Parser) Parse() (Queryer, error) {
	return p.parse()
}

func (p *Parser) parse() (Queryer, error) {
	var qs []Queryer
	for !p.isDone() {
		q, err := p.parseQuery()
		if err != nil {
			return nil, err
		}
		qs = append(qs, q)
		switch p.curr.Type {
		case TokComma:
			p.next()
			switch {
			case p.curr.isKey() || p.curr.isLevel() || p.curr.isType():
			default:
				return nil, fmt.Errorf("parse: unexpected token %s", p.curr)
			}
		case TokEOF:
		default:
			return nil, fmt.Errorf("parse: unexpected token %s", p.curr)
		}
	}
	var q Queryer
	switch len(qs) {
	case 0:
		return nil, ErrEmpty
	case 1:
		q = qs[0]
	default:
		q = Queryset(qs)
	}
	return q, nil
}

func (p *Parser) parseQuery() (Queryer, error) {
	var q Query
	q.depth = TokLevelAny
	if p.curr.isLevel() {
		q.depth = p.curr.Type
		p.next()
	}
	choices, err := p.parseChoices()
	if err != nil {
		return nil, err
	}
	q.choices = choices
	if p.curr.isSelector() {
		get, err := p.parseSelector()
		if err != nil {
			return nil, err
		}
		q.get = get
	}
	if p.curr.isExpression() {
		p.next()
		match, err := p.parseMatcher()
		if err != nil {
			return nil, err
		}
		q.match = match
	}
	if p.curr.isLevel() {
		qs, err := p.parseQuery()
		if err != nil {
			return nil, err
		}
		q.next = qs
	}
	return q, nil
}

func (p *Parser) parseChoices() ([]Accepter, error) {
	var kind rune
	if p.curr.isType() {
		kind = p.curr.Type
		p.next()
	}
	if p.curr.isKey() {
		var a Accepter
		if p.curr.Type == TokPattern {
			a = Pattern{
				pattern: p.curr.Literal,
				kind:    kind,
			}
		} else {
			a = Name{
				label: p.curr.Literal,
				kind:  kind,
			}
		}
		p.next()
		return []Accepter{a}, nil
	}
	if p.curr.Type != TokBegGrp {
		return nil, fmt.Errorf("choices: unexpected token %s, want lparen", p.curr)
	}
	p.next()
	var choices []Accepter
	for !p.isDone() && p.curr.Type != TokEndGrp {
		if !p.curr.isKey() {
			return nil, fmt.Errorf("choices: unexpected token %s, want identifier", p.curr)
		}
		var a Accepter
		if p.curr.Type == TokPattern {
			a = Pattern{
				pattern: p.curr.Literal,
				kind:    kind,
			}
		} else {
			a = Name{
				label: p.curr.Literal,
				kind:  kind,
			}
		}
		choices = append(choices, a)
		p.next()
		switch p.curr.Type {
		case TokComma:
			p.next()
		case TokEndGrp:
		default:
			return nil, fmt.Errorf("choices: unexpected token %s, want comma, rparen", p.curr)
		}
	}
	if p.curr.Type != TokEndGrp {
		return nil, fmt.Errorf("choices: unexpected token %s, want rparen", p.curr)
	}
	p.next()
	return choices, nil
}

func (p *Parser) parseMatcher() (Matcher, error) {
	var (
		left Matcher
		err  error
	)
	if p.curr.Type == TokBegGrp {
		p.next()
		left, err = p.parseMatcher()
		if err == nil && p.curr.Type != TokEndGrp {
			return nil, fmt.Errorf("expr: unexpected token %s, want )", p.curr)
		}
		p.next()
	} else {
		left, err = p.parseExpression()
	}
	if err != nil {
		return nil, err
	}
	if p.curr.isRelation() {
		i := Infix{
			op:   p.curr.Type,
			left: left,
		}
		p.next()
		right, err := p.parseMatcher()
		if err != nil {
			return nil, err
		}
		i.right = right
		return i, nil
	}
	switch p.curr.Type {
	case TokEndExpr:
		p.next()
	case TokEndGrp:
	default:
		return nil, fmt.Errorf("expr: unexpected token %s, want rsquare|rparen", p.curr)
	}
	return left, err
}

func (p *Parser) parseEval() (Func, error) {
	p.next()
	if p.curr.Type != TokLiteral {
		return nil, fmt.Errorf("eval: unexpected token %s, want identifier", p.curr)
	}
	fn, ok := funcnames[p.curr.Literal]
	if !ok {
		return nil, fmt.Errorf("eval: unknown function %q", p.curr.Literal)
	}
	p.next()
	var args []interface{}
	if p.curr.Type == TokBegGrp {
		p.next()
		for !p.isDone() && p.curr.Type != TokEndGrp {
			if !p.curr.isValue() {
				return nil, fmt.Errorf("eval: unexpected token %s, want 'value'", p.curr)
			}
			arg, err := p.convertValue()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			p.next()
			switch p.curr.Type {
			case TokComma:
				p.next()
			case TokEndGrp:
			default:
				return nil, fmt.Errorf("eval: unexpected token %s, want ')' or ','", p.curr)
			}
		}
		if p.curr.Type != TokEndGrp {
			return nil, fmt.Errorf("eval: unexpected token %s, want ')'", p.curr)
		}
		p.next()
	}
	return fn(args), nil
}

func (p *Parser) parseExpression() (Matcher, error) {
	var left Matcher
	if !p.curr.isKey() {
		return nil, fmt.Errorf("expr: unexpected token %s, want identifier", p.curr)
	}
	var (
		ident = p.curr
		eval  Func
	)
	p.next()
	if p.curr.Type == TokLevelOne {
		fn, err := p.parseEval()
		if err != nil {
			return nil, err
		}
		if !p.curr.isComparison() {
			return nil, fmt.Errorf("expr: unexpected token %s, want 'cmp' after function call", p.curr)
		}
		eval = fn
	}
	if p.curr.isComparison() {
		e := Expr{
			option: ident.Literal,
			op:     p.curr.Type,
			eval:   eval,
		}
		p.next()
		if !p.curr.isValue() && p.curr.Type != TokBegGrp {
			return nil, fmt.Errorf("expr: unexpected token %s, want value", p.curr)
		}
		value, err := p.parseValue(e.op)
		if err != nil {
			return nil, fmt.Errorf("expr(value): %w", err)
		}
		e.value = value
		left = e
		p.next()
	} else {
		left = Has{option: ident.Literal}
	}
	return left, nil
}

var timestr = []string{
	"15:04:05",
	"15:04:05.000",
	"15:04:05.000000",
}
var datestr = []string{
	"2006-01-02T15:04:05",
	"2006-01-02T15:04:05.000",
	"2006-01-02T15:04:05.000000",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05.000Z07:00",
	"2006-01-02T15:04:05.000000Z07:00",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04:05.000",
	"2006-01-02 15:04:05.000000",
	"2006-01-02 15:04:05Z07:00",
	"2006-01-02 15:04:05.000Z07:00",
	"2006-01-02 15:04:05.000000Z07:00",
}

func (p *Parser) convertValue() (interface{}, error) {
	var (
		val interface{}
		err error
	)
	switch p.curr.Type {
	case TokPattern:
		val = p.curr.Literal
	case TokLiteral:
		val = p.curr.Literal
	case TokBool:
		val, err = strconv.ParseBool(p.curr.Literal)
	case TokFloat:
		val, err = strconv.ParseFloat(p.curr.Literal, 64)
	case TokInteger:
		val, err = strconv.ParseInt(p.curr.Literal, 0, 64)
	case TokTime:
		for _, str := range timestr {
			val, err = time.Parse(str, p.curr.Literal)
			if err == nil {
				break
			}
		}
	case TokDate:
		val, err = time.Parse("2006-01-02", p.curr.Literal)
	case TokDateTime:
		for _, str := range datestr {
			val, err = time.Parse(str, p.curr.Literal)
			if err == nil {
				break
			}
		}
	default:
		err = fmt.Errorf("unknown value type: %s", p.curr)
	}
	return val, err
}

func (p *Parser) parseValue(op rune) (interface{}, error) {
	do := func() (interface{}, error) {
		if op == TokMatch && p.curr.Type != TokPattern {
			return nil, fmt.Errorf("value: unexpected token %s, want pattern", p.curr)
		}
		return p.convertValue()
	}
	if p.curr.isValue() {
		return do()
	}
	if p.curr.Type != TokBegGrp {
		return nil, fmt.Errorf("value: unexpected token %s, want begin", p.curr)
	}
	p.next()

	var values []interface{}
	for !p.isDone() && p.curr.Type != TokEndGrp {
		val, err := do()
		if err != nil {
			return nil, err
		}
		values = append(values, val)
		p.next()
		switch p.curr.Type {
		case TokComma:
			p.next()
		case TokEndGrp:
		default:
			return nil, fmt.Errorf("value: unexpected token %s, want comma|end", p.curr)
		}
	}
	if p.curr.Type != TokEndGrp {
		return nil, fmt.Errorf("value: unexpected token %s, want end", p.curr)
	}
	return values, nil
}

func (p *Parser) parseSelector() (Selector, error) {
	parse, ok := p.selectors[p.curr.Type]
	if !ok {
		return nil, fmt.Errorf("selector: unknown selector %s", p.curr.Literal)
	}
	curr := p.curr
	p.next()
	return parse(curr.Type)
}

func (p *Parser) parseSelectSimple(curr rune) (Selector, error) {
	var (
		get Selector
		err error
	)
	switch curr {
	case TokSelectFirst:
		get = First{}
	case TokSelectLast:
		get = Last{}
	case TokSelectInt:
		get = Int{}
	case TokSelectFloat:
		get = Float{}
	case TokSelectNumber:
		get = Number{}
	case TokSelectBool:
		get = Boolean{}
	case TokSelectString:
		get = String{}
	case TokSelectTruthy:
		get = Truthy{}
	case TokSelectFalsy:
		get = Falsy{}
	default:
		err = fmt.Errorf("selector: unsupported token %s", p.curr)
	}
	return get, err
}

func (p *Parser) parseSelectAt(_ rune) (Selector, error) {
	var at At
	if p.curr.Type != TokBegGrp {
		return nil, fmt.Errorf("at: unexpected token %s, want lparen", p.curr)
	}
	p.next()
	ix, err := strconv.ParseInt(p.curr.Literal, 0, 64)
	if err != nil {
		return nil, fmt.Errorf("at: %w", err)
	}
	at.index = int(ix)

	p.next()
	if p.curr.Type != TokEndGrp {
		return nil, fmt.Errorf("at: unexpected token %s, want rparen", p.curr)
	}
	p.next()
	return at, nil
}

func (p *Parser) parseSelectRange(_ rune) (Selector, error) {
	var rg Range
	if p.curr.Type != TokBegGrp {
		return nil, fmt.Errorf("range: unexpected token %s, want lparen", p.curr)
	}
	p.next()
	if p.curr.Type == TokInteger {
		ix, err := strconv.ParseInt(p.curr.Literal, 0, 64)
		if err != nil {
			return nil, fmt.Errorf("range: %w", err)
		}
		rg.start = int(ix)
		p.next()
	}
	if p.curr.Type != TokComma {
		return nil, fmt.Errorf("range: unexpected token %s, want comma", p.curr)
	}
	p.next()
	if p.curr.Type == TokInteger {
		ix, err := strconv.ParseInt(p.curr.Literal, 0, 64)
		if err != nil {
			return nil, err
		}
		rg.end = int(ix)
		p.next()
	}
	if p.curr.Type != TokEndGrp {
		return nil, fmt.Errorf("range: unexpected token %s, want rparen", p.curr)
	}
	p.next()
	return rg, nil
}

func (p *Parser) isDone() bool {
	return p.curr.isDone()
}

func (p *Parser) next() {
	if p.curr.Type == TokEOF {
		return
	}
	p.curr = p.peek
	p.peek = p.scan.Scan()
}
