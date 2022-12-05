package query

import (
	"fmt"
	"strconv"
	"unicode/utf8"
)

type Parser struct {
	scan *Scanner
	curr Token
	peek Token
}

func Parse(str string) (F, error) {
	// if str == "." {
	// 	ch := chain{
	// 		queries: []Query{keepAll},
	// 	}
	// 	return &ch, nil
	// }
	p := Parser{
		scan: Scan(str),
	}
	p.next()
	p.next()
	return p.Parse()
}

func (p *Parser) Parse() (F, error) {
	return p.parseChain()
}

func (p *Parser) parseChain() (F, error) {
	var ch chain
	for !p.done() && !p.is(Pipe) {
		q, err := p.parse()
		if err != nil {
			return nil, err
		}
		ch.queries = append(ch.queries, q)
		switch p.curr.Type {
		case Pipe:
			p.next()
			if p.is(Eof) {
				return nil, fmt.Errorf("parser: expected query after '|'")
			}
		case Eof:
		default:
			return nil, fmt.Errorf("parser: expected '|' or eof")
		}
	}
	if !p.is(Eof) {
		return nil, fmt.Errorf("parser: expected end of input")
	}
	return &ch, nil
}

func (p *Parser) parse() (Query, error) {
	if p.is(Dot) && (p.peekIs(Pipe) || p.peekIs(Eof)) {
		p.next()
		return keepAll, nil
	}
	var list []Query
	for !p.done() && !p.is(Pipe) {
		curr, err := p.parseQuery()
		if err != nil {
			return nil, err
		}
		list = append(list, curr)
		switch p.curr.Type {
		case Comma:
			p.next()
			if p.is(Eof) {
				return nil, fmt.Errorf("parser: expected query after ','")
			}
		case Eof, Pipe:
		default:
			return nil, fmt.Errorf("parser: expected ',' or eof")
		}
	}
	if len(list) == 1 {
		return list[0], nil
	}
	a := any{
		list: list,
	}
	return &a, nil
}

func (p *Parser) parseQuery() (Query, error) {
	var (
		curr Query
		err  error
		dot  bool
	)
	switch p.curr.Type {
	default:
		return nil, fmt.Errorf("query: expected '.', '[' or '{'")
	case Dot:
		dot = true
		p.next()
	case Lsquare:
		curr, err = p.parseArray()
	case Lcurly:
		curr, err = p.parseObject()
	}

	if dot {
		switch p.curr.Type {
		case Literal:
			curr, err = p.parseIdent()
		case Lsquare:
			curr, err = p.parseIndex()
		default:
			return nil, fmt.Errorf("query: expected '.' or '('")
		}
	}
	if err != nil {
		return nil, err
	}
	switch p.curr.Type {
	case Eof, Comma, Rsquare, Rcurly, Pipe:
	default:
		return nil, fmt.Errorf("query: expected ',' or end of input")
	}
	return curr, err
}

func (p *Parser) parseArray() (Query, error) {
	p.next()
	var arr array
	for !p.done() && !p.is(Rsquare) {
		q, err := p.parseQuery()
		if err != nil {
			return nil, err
		}
		arr.list = append(arr.list, q)
		switch p.curr.Type {
		case Comma:
			p.next()
			if p.is(Rsquare) {
				return nil, fmt.Errorf("array constructor: expected query after comma")
			}
		case Rsquare:
		default:
			return nil, fmt.Errorf("array constructor: expected ',' or '}'")
		}
	}
	if err := p.expect(Rsquare, "array constructor: expected ']' at end"); err != nil {
		return nil, err
	}
	p.next()
	return &arr, nil
}

func (p *Parser) parseObject() (Query, error) {
	p.next()
	obj := object{
		fields: make(map[string]Query),
	}
	for !p.done() && !p.is(Rcurly) {
		var ident string
		switch p.curr.Type {
		case Dot:
			ident = p.peek.Literal
		case Literal:
			ident = p.curr.Literal
			p.next()
			if err := p.expect(Colon, "object: expect ':' after literal"); err != nil {
				return nil, err
			}
			p.next()
		default:
			return nil, fmt.Errorf("object: expected '.' or literal")
		}
		q, err := p.parseQuery()
		if err != nil {
			return nil, err
		}
		obj.fields[ident] = q
		switch p.curr.Type {
		case Comma:
			p.next()
			if p.is(Rcurly) {
				return nil, fmt.Errorf("object: expected query after comma")
			}
		case Rcurly:
		default:
			return nil, fmt.Errorf("object: expected ',' or '}'")
		}
	}
	if err := p.expect(Rcurly, "object: expected '}' at end"); err != nil {
		return nil, err
	}
	p.next()
	switch p.curr.Type {
	case Comma, Eof, Rsquare, Rcurly, Pipe:
	default:
		return nil, fmt.Errorf("object: unexpected character %s", p.curr)
	}
	return &obj, nil
}

func (p *Parser) parseIdent() (Query, error) {
	var (
		id  ident
		err error
	)
	id.ident = p.curr.Literal
	p.next()
	switch p.curr.Type {
	case Dot:
		id.next, err = p.parseQuery()
	case Lsquare:
		id.next, err = p.parseIndex()
	case Comma, Eof, Rcurly, Rsquare, Pipe:
	default:
		err = fmt.Errorf("identifier: unexpected character %s", p.curr)
	}
	return &id, err
}

func (p *Parser) parseIndex() (Query, error) {
	p.next()
	var (
		idx index
		err error
	)
	for !p.done() && !p.is(Rsquare) {
		if err := p.expect(Number, "index: number expected"); err != nil {
			return nil, err
		}

		if _, err := strconv.Atoi(p.curr.Literal); err != nil {
			return nil, err
		}
		idx.list = append(idx.list, p.curr.Literal)
		p.next()
		switch p.curr.Type {
		case Comma:
			p.next()
			if p.is(Rsquare) {
				return nil, fmt.Errorf("index: expected number after ','")
			}
		case Rsquare:
		default:
			return nil, fmt.Errorf("index: expected ',' or ']")
		}
	}
	if err := p.expect(Rsquare, "index: expected ']"); err != nil {
		return nil, err
	}
	p.next()
	switch p.curr.Type {
	case Dot:
		idx.next, err = p.parseQuery()
	case Comma, Eof, Rsquare, Rcurly, Pipe:
	default:
		err = fmt.Errorf("index: unexpected character %s", p.curr)
	}
	return &idx, err
}

func (p *Parser) expect(kind rune, msg string) error {
	if p.curr.Type != kind {
		return fmt.Errorf(msg)
	}
	return nil
}

func (p *Parser) peekIs(kind rune) bool {
	return p.peek.Type == kind
}

func (p *Parser) is(kind rune) bool {
	return p.curr.Type == kind
}

func (p *Parser) done() bool {
	return p.curr.Type == Eof
}

func (p *Parser) next() {
	p.curr = p.peek
	p.peek = p.scan.Scan()
}

const (
	Eof rune = -(1 + iota)
	Literal
	Number
	Dot
	Recurse
	Comma
	Lparen
	Rparen
	Lsquare
	Rsquare
	Lcurly
	Rcurly
	Colon
	Pipe
	Invalid
)

type Token struct {
	Literal string
	Type    rune
}

func (t Token) String() string {
	switch t.Type {
	case Eof:
		return "<eof>"
	case Dot:
		return "<dot>"
	case Recurse:
		return "<recurse>"
	case Comma:
		return "<comma>"
	case Lparen:
		return "<lparen>"
	case Rparen:
		return "<rparen>"
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
	case Pipe:
		return "<pipe>"
	case Invalid:
		if t.Literal != "" {
			return fmt.Sprintf("invalid(%s)", t.Literal)
		}
		return "<invalid>"
	case Literal:
		return fmt.Sprintf("literal(%s)", t.Literal)
	case Number:
		return fmt.Sprintf("number(%s)", t.Literal)
	default:
		return "<unknown>"
	}
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
	case isDelim(s.char):
		s.scanDelim(&tok)
	case isBlank(s.char):
		s.skipBlank()
		return s.Scan()
	default:
	}
	return tok
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
	quote := s.char
	s.read()
	pos := s.curr
	for !s.done() && s.char != quote {
		s.read()
	}
	tok.Type = Literal
	if s.char != quote {
		tok.Type = Invalid
	}
	tok.Literal = string(s.input[pos:s.curr])
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
		tok.Type = Dot
		if s.peek() == s.char {
			s.read()
			tok.Type = Recurse
		}
	case '(':
		tok.Type = Lparen
	case ')':
		tok.Type = Rparen
	case '[':
		tok.Type = Lsquare
	case ']':
		tok.Type = Rsquare
	case '|':
		tok.Type = Pipe
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

func isGroup(r rune) bool {
	return r == '(' || r == ')' || r == '[' || r == ']' || r == '{' || r == '}'
}

func isPunct(r rune) bool {
	return r == '.' || r == ',' || r == ':' || r == '|'
}

func isDelim(r rune) bool {
	return isGroup(r) || isPunct(r)
}
