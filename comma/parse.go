package comma

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/midbel/slices"
)

type Parser struct {
	scan *Scanner
	curr Token
	peek Token

	prefix map[rune]func() (Indexer, error)
	infix  map[rune]func(Indexer) (Indexer, error)

	stack *slices.Stack[rune]
}

func Parse(str string) (Indexer, error) {
	p := Parser{
		scan:  Scan(strings.TrimSpace(str)),
		stack: slices.New[rune](),
	}
	p.prefix = map[rune]func() (Indexer, error){
		Sub:     p.parseUnary,
		Not:     p.parseUnary,
		Index:   p.parseUnary,
		Number:  p.parseUnary,
		Literal: p.parseUnary,
		Lparen:  p.parseGroup,
	}
	p.infix = map[rune]func(Indexer) (Indexer, error){
		Add:      p.parseBinary,
		Sub:      p.parseBinary,
		Mul:      p.parseBinary,
		Div:      p.parseBinary,
		Pow:      p.parseBinary,
		Mod:      p.parseBinary,
		Question: p.parseTernary,
	}
	p.next()
	p.next()
	return p.Parse()
}

func (p *Parser) Parse() (Indexer, error) {
	return p.parse()
}

func (p *Parser) parse() (Indexer, error) {
	var list []Indexer
	for !p.done() {
		i, err := p.parseSingle()
		if err != nil {
			return nil, err
		}
		list = append(list, i)
		switch p.curr.Type {
		case Comma:
			p.next()
			if p.is(Eof) {
				return nil, fmt.Errorf("parse: unexpected end of input after ','")
			}
		case Eof:
		default:
			return nil, fmt.Errorf("parse: expected ',' or end of input")
		}
	}
	if len(list) == 1 {
		return list[0], nil
	}
	g := group{
		list: list,
	}
	return &g, nil
}

func (p *Parser) parseSingle() (Indexer, error) {
	switch p.curr.Type {
	case Lcurly:
		return p.parseObject()
	case Lsquare:
		return p.parseArray()
	default:
		return p.parseIndexer()
	}
}

func (p *Parser) parseIndexer() (Indexer, error) {
	if p.peekIs(Range) || p.peekIs(RangeAdd) {
		return p.parseRange()
	}
	return p.parseExpression(bindLowest)
}

func (p *Parser) parseRange() (Indexer, error) {
	if err := p.expect(Index, "range: expected '$'"); err != nil {
		return nil, err
	}
	beg, err := strconv.Atoi(p.curr.Literal)
	if err != nil {
		return nil, err
	}
	p.next()

	add := p.is(RangeAdd)

	p.next()
	if err := p.expect(Index, "index: expected '$' after '.."); err != nil {
		return nil, err
	}
	end, err := strconv.Atoi(p.curr.Literal)
	if err != nil {
		return nil, err
	}
	rg := interval{
		beg:  beg,
		end:  end,
		add:  add,
		flat: p.stack.Top() == Lsquare,
	}
	p.next()
	return &rg, nil
}

func (p *Parser) parseObject() (Indexer, error) {
	p.stack.Push(Lcurly)
	defer p.stack.Pop()

	p.next()
	var obj object
	obj.fields = make(map[string]Indexer)
	for !p.done() && !p.is(Rcurly) {
		if err := p.expect(Literal, "object: expected literal"); err != nil {
			return nil, err
		}
		ident := p.curr.Literal
		p.next()
		if err := p.expect(Colon, "object: expected ':'"); err != nil {
			return nil, err
		}
		p.next()
		ix, err := p.parseSingle()
		if err != nil {
			return nil, err
		}
		obj.fields[ident] = ix
		obj.keys = append(obj.keys, ident)
		switch p.curr.Type {
		case Comma:
			p.next()
			if p.is(Rcurly) {
				return nil, p.parseError("object: expected key after comma, not '}")
			}
		case Rcurly:
		default:
			return nil, p.parseError("object: expected ',' or '}")
		}
	}
	if err := p.expect(Rcurly, "object: expected '}"); err != nil {
		return nil, err
	}
	p.next()
	return &obj, nil
}

func (p *Parser) parseArray() (Indexer, error) {
	p.stack.Push(Lsquare)
	defer p.stack.Pop()

	p.next()
	var arr array
	for !p.done() && !p.is(Rsquare) {
		ix, err := p.parseSingle()
		if err != nil {
			return nil, err
		}
		arr.list = append(arr.list, ix)
		switch p.curr.Type {
		case Comma:
			p.next()
			if p.is(Rsquare) {
				return nil, p.parseError("array: expected key after comma, not ']")
			}
		case Rsquare:
		default:
			return nil, p.parseError("array: expected ',' or ']")
		}
	}
	if err := p.expect(Rsquare, "array: expected ']"); err != nil {
		return nil, err
	}
	p.next()
	return &arr, nil
}

func (p *Parser) exprDone() bool {
	return p.done() || p.is(Comma) || p.is(Rcurly) || p.is(Rsquare)
}

func (p *Parser) parseExpression(bind int) (Indexer, error) {
	left, err := p.parsePrefix()
	if err != nil {
		return nil, err
	}
	for !p.exprDone() && bind < bindings.Get(p.curr.Type) {
		left, err = p.parseInfix(left)
		if err != nil {
			return nil, err
		}
	}
	return left, nil
}

func (p *Parser) parsePrefix() (Indexer, error) {
	fn, ok := p.prefix[p.curr.Type]
	if !ok {
		return nil, p.parseError("token can not be parsed as prefix")
	}
	return fn()
}

func (p *Parser) parseInfix(left Indexer) (Indexer, error) {
	fn, ok := p.infix[p.curr.Type]
	if !ok {
		return nil, p.parseError("token can not be parsed as infix")
	}
	return fn(left)
}

func (p *Parser) parseTernary(left Indexer) (Indexer, error) {
	p.next()
	test := ternary{
		cdt: left,
	}
	csq, err := p.parseExpression(bindLowest)
	if err != nil {
		return nil, err
	}
	if err := p.expect(Colon, "ternary: expected ':'"); err != nil {
		return nil, err
	}
	p.next()

	alt, err := p.parseExpression(bindLowest)
	if err != nil {
		return nil, err
	}
	test.csq = csq
	test.alt = alt
	return &test, nil
}

func (p *Parser) parseBinary(left Indexer) (Indexer, error) {
	bin := binary{
		left: left,
		op:   p.curr.Type,
	}
	p.next()
	right, err := p.parseExpression(bindings.Get(bin.op))
	if err != nil {
		return nil, err
	}
	bin.right = right
	return &bin, nil
}

func (p *Parser) parseGroup() (Indexer, error) {
	p.next()
	ix, err := p.parseExpression(bindLowest)
	if err != nil {
		return nil, err
	}
	if err := p.expect(Rparen, "group: expected ')'"); err != nil {
		return nil, err
	}
	p.next()
	return ix, nil
}

func (p *Parser) parseUnary() (Indexer, error) {
	var ix Indexer
	switch p.curr.Type {
	case Sub:
		p.next()
		right, err := p.parseExpression(bindPrefix)
		if err != nil {
			return nil, err
		}
		ix = &unary{
			op:    Sub,
			right: right,
		}
	case Index:
		n, err := strconv.Atoi(p.curr.Literal)
		if err != nil {
			return nil, err
		}
		ix = &index{
			index: n,
		}
		p.next()
	case Number, Literal:
		ix = &literal{
			value: p.curr.Literal,
		}
		p.next()
	default:
		return nil, p.parseError("unsupported unary token")
	}
	return ix, nil
}

func (p *Parser) done() bool {
	return p.is(Eof)
}

func (p *Parser) next() {
	p.curr = p.peek
	p.peek = p.scan.Scan()
}

func (p *Parser) is(kind rune) bool {
	return p.curr.Type == kind
}

func (p *Parser) peekIs(kind rune) bool {
	return p.peek.Type == kind
}

func (p *Parser) expect(kind rune, msg string) error {
	if !p.is(kind) {
		return p.parseError(msg)
	}
	return nil
}

func (p *Parser) parseError(msg string, args ...interface{}) error {
	return fmt.Errorf(msg, args...)
}

type Token struct {
	Literal string
	Type    rune
}

func (t Token) String() string {
	switch t.Type {
	case Eof:
		return "<eof>"
	case Not:
		return "<not>"
	case Range:
		return "<range>"
	case RangeAdd:
		return "<range-add>"
	case Add:
		return "<add>"
	case Sub:
		return "<subtract>"
	case Div:
		return "<divide>"
	case Mul:
		return "<multiply>"
	case Mod:
		return "<modulo>"
	case Pow:
		return "<power>"
	case Comma:
		return "<comma>"
	case Lsquare:
		return "<lsquare>"
	case Rsquare:
		return "<rsquare>"
	case Lcurly:
		return "<lcurly>"
	case Rcurly:
		return "<rcurly>"
	case Colon:
		return "<colon>"
	case Question:
		return "<question>"
	case Invalid:
		if t.Literal != "" {
			return fmt.Sprintf("invalid(%s)", t.Literal)
		}
		return "<invalid>"
	case Index:
		return fmt.Sprintf("index(%s)", t.Literal)
	case Literal:
		return fmt.Sprintf("literal(%s)", t.Literal)
	case Number:
		return fmt.Sprintf("number(%s)", t.Literal)
	default:
		return "<unknown>"
	}
}

const (
	Eof = -(1 + iota)
	Literal
	Number
	Index
	Comma
	Lsquare
	Rsquare
	Lcurly
	Rcurly
	Lparen
	Rparen
	Colon
	Range
	RangeAdd
	Add
	Sub
	Mul
	Div
	Pow
	Mod
	Not
	Question
	Invalid
)

type bindmap map[rune]int

var bindings = bindmap{
	Add: bindAdd,
	Sub: bindAdd,
	Mul: bindMul,
	Div: bindMul,
	Pow: bindMul,
	Mod: bindMul,
}

const (
	bindLowest = iota
	bindAdd
	bindMul
	bindPrefix
)

func (b bindmap) Get(k rune) int {
	p, ok := b[k]
	if !ok {
		p = bindLowest
	}
	return p
}

type Scanner struct {
	input []byte
	curr  int
	next  int
	char  rune
}

func Scan(str string) *Scanner {
	return &Scanner{
		input: []byte(str),
	}
}

func (s *Scanner) Scan() Token {
	var tok Token
	s.read()
	if s.done() {
		tok.Type = Eof
		return tok
	}
	switch {
	case isLetter(s.char):
		s.scanIdent(&tok)
	case isQuote(s.char):
		s.scanQuote(&tok)
	case isDigit(s.char):
		s.scanNumber(&tok)
	case isOperator(s.char):
		s.scanOperator(&tok)
	case isDelim(s.char):
		s.scanDelim(&tok)
	case isIndex(s.char):
		s.scanIndex(&tok)
	case isBlank(s.char):
		s.skipBlank()
		return s.Scan()
	default:
		fmt.Printf("unknown????: %c %02[1]x\n", s.char)
	}
	return tok
}

func (s *Scanner) scanIndex(tok *Token) {
	s.read()
	s.scanNumber(tok)
	if tok.Type == Number {
		tok.Type = Index
	}
}

func (s *Scanner) scanIdent(tok *Token) {
	defer s.unread()

	pos := s.curr
	for !s.done() && isAlpha(s.char) {
		s.read()
	}
	tok.Type = Literal
	tok.Literal = string(s.input[pos:s.curr])
}

func (s *Scanner) scanQuote(tok *Token) {
	var (
		quote = s.char
		pos   = s.curr
	)
	s.read()
	for !s.done() && s.char != quote {
		s.read()
	}
	tok.Type = Literal
	if s.char != quote {
		tok.Type = Invalid
	}
	tok.Literal = string(s.input[pos:s.next])
}

func (s *Scanner) scanNumber(tok *Token) {
	defer s.unread()

	pos := s.curr
	for !s.done() && isDigit(s.char) {
		s.read()
	}
	tok.Type = Number
	tok.Literal = string(s.input[pos:s.curr])
}

func (s *Scanner) scanOperator(tok *Token) {
	switch s.char {
	case '+':
		tok.Type = Add
		if s.check('.', '.') {
			tok.Type = RangeAdd
			s.read()
			s.read()
		}
	case '-':
		tok.Type = Sub
	case '*':
		tok.Type = Mul
		if k := s.peek(); k == s.char {
			tok.Type = Pow
			s.read()
		}
	case '/':
		tok.Type = Div
	case '%':
		tok.Type = Mod
	case '!':
		tok.Type = Not
	case '?':
		tok.Type = Question
	default:
		tok.Type = Invalid
	}
}

func (s *Scanner) scanDelim(tok *Token) {
	switch s.char {
	case '{':
		tok.Type = Lcurly
	case '}':
		tok.Type = Rcurly
	case ':':
		tok.Type = Colon
	case ',':
		tok.Type = Comma
	case '.':
		tok.Type = Invalid
		if k := s.peek(); k == s.char {
			tok.Type = Range
			s.read()
		}
	case '[':
		tok.Type = Lsquare
	case ']':
		tok.Type = Rsquare
	case '(':
		tok.Type = Lparen
	case ')':
		tok.Type = Rparen
	default:
		tok.Type = Invalid
	}
}

func (s *Scanner) skipBlank() {
	defer s.unread()
	for !s.done() && isBlank(s.char) {
		s.read()
	}
}

func (s *Scanner) read() {
	if s.curr >= len(s.input) {
		s.char = 0
		return
	}
	c, z := utf8.DecodeRune(s.input[s.next:])
	s.curr = s.next
	s.next = s.curr + z
	s.char = c
}

func (s *Scanner) unread() {
	c, z := utf8.DecodeRune(s.input[s.curr:])
	s.char = c
	s.next = s.curr
	s.curr -= z
}

func (s *Scanner) peek() rune {
	c, _ := utf8.DecodeRune(s.input[s.next:])
	return c
}

func (s *Scanner) check(vs ...rune) bool {
	next := s.next
	for i := range vs {
		if next >= len(s.input) {
			return false
		}
		r, z := utf8.DecodeRune(s.input[next:])
		if r != vs[i] {
			return false
		}
		next += z
	}
	return true
}

func (s *Scanner) done() bool {
	return s.curr >= len(s.input)
}

func isAlpha(r rune) bool {
	return isLower(r) || isUpper(r) || isDigit(r) || r == '_'
}

func isLetter(r rune) bool {
	return isLower(r) || isUpper(r)
}

func isLower(r rune) bool {
	return r >= 'a' && r <= 'z'
}

func isUpper(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isBlank(r rune) bool {
	return r == ' ' || r == '\t'
}

func isQuote(r rune) bool {
	return r == '\'' || r == '"'
}

func isIndex(r rune) bool {
	return r == '$'
}

func isOperator(r rune) bool {
	return r == '+' || r == '-' || r == '*' || r == '%' || r == '/' || r == '!' || r == '?'
}

func isDelim(r rune) bool {
	return isPunct(r) || isGroup(r)
}

func isPunct(r rune) bool {
	return r == ',' || r == ':' || r == '.'
}

func isGroup(r rune) bool {
	return r == '[' || r == ']' || r == '{' || r == '}' || r == '(' || r == ')'
}
