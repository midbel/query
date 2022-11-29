package query

import (
	"fmt"
	"strconv"
	"unicode/utf8"
)

type Query interface {
	Next(string) (Query, error)
}

type Parser struct {
	scan *Scanner
	curr Token
	peek Token
}

func Parse(str string) (Query, error) {
	if str == "." {
		return KeepAll, nil
	}
	p := Parser{
		scan: Scan(str),
	}
	p.next()
	p.next()
	return p.Parse()
}

func (p *Parser) Parse() (Query, error) {
	return p.parse()
}

func (p *Parser) parse() (Query, error) {
	var list []Query
	for !p.done() {
		curr, err := p.parseFilter()
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
		case Eof:
		default:
			return nil, fmt.Errorf("parser: expected ',' or eof")
		}
	}
	if len(list) == 0 {
		return list[0], nil
	}
	a := any{
		list: list,
	}
	return &a, nil
}

func (p *Parser) parseFilter() (Query, error) {
	var (
		curr Query
		err  error
	)
	if err := p.expect(Dot, "parser: expected '.'"); err != nil {
		return nil, err
	}
	p.next()
	switch p.curr.Type {
	case Literal:
		curr, err = p.parseIdent()
	case Lparen:
		curr, err = p.parseGroup()
	case Lsquare:
		curr, err = p.parseArray()
	default:
		return nil, fmt.Errorf("expected '.' or '('")
	}
	if err != nil {
		return nil, err
	}
	switch p.curr.Type {
	case Eof, Rparen, Comma:
	default:
		return nil, fmt.Errorf("expected ',' or end of input")
	}
	return curr, err
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
		id.next, err = p.parseFilter()
	case Lsquare:
		id.next, err = p.parseArray()
	case Comma, Eof, Rparen:
	default:
		err = fmt.Errorf("identifier: unexpected character %s", p.curr)
	}
	return &id, err
}

func (p *Parser) parseArray() (Query, error) {
	p.next()
	var (
		arr array
		err error
	)
	for !p.done() && !p.is(Rsquare) {
		if err := p.expect(Number, "array: number expected"); err != nil {
			return nil, err
		}

		if _, err := strconv.Atoi(p.curr.Literal); err != nil {
			return nil, err
		}
		arr.index = append(arr.index, p.curr.Literal)
		p.next()
		switch p.curr.Type {
		case Comma:
			p.next()
			if p.is(Rsquare) {
				return nil, fmt.Errorf("array: expected number after ','")
			}
		case Rsquare:
		default:
			return nil, fmt.Errorf("array: expected ',' or ']")
		}
	}
	if err := p.expect(Rsquare, "array: expected ']"); err != nil {
		return nil, err
	}
	p.next()
	switch p.curr.Type {
	case Dot:
		arr.next, err = p.parseFilter()
	case Comma, Eof, Rparen:
	default:
		err = fmt.Errorf("array: unexpected character %s", p.curr)
	}
	return &arr, err
}

func (p *Parser) parseGroup() (Query, error) {
	p.next()
	var (
		grp group
		err error
	)
	for !p.done() && !p.is(Rparen) {
		curr, err := p.parseFilter()
		if err != nil {
			return nil, err
		}
		grp.list = append(grp.list, curr)
		switch p.curr.Type {
		case Comma:
			p.next()
			if p.is(Rparen) {
				return nil, fmt.Errorf("group: expected query after ','")
			}
		case Rparen:
		default:
			return nil, fmt.Errorf("group: expected ',' or ')")
		}
	}
	if err := p.expect(Rparen, "group: expected ')'"); err != nil {
		return nil, err
	}
	p.next()
	switch p.curr.Type {
	case Dot:
		grp.next, err = p.parseFilter()
	case Lsquare:
		grp.next, err = p.parseArray()
	case Comma, Eof:
	default:
		err = fmt.Errorf("group: unexpected character %s", p.curr)
	}
	return &grp, err
}

func (p *Parser) expect(kind rune, msg string) error {
	if p.curr.Type != kind {
		return fmt.Errorf(msg)
	}
	return nil
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
	Comma
	Lparen
	Rparen
	Lsquare
	Rsquare
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
	case ',':
		tok.Type = Comma
	case '.':
		tok.Type = Dot
	case '(':
		tok.Type = Lparen
	case ')':
		tok.Type = Rparen
	case '[':
		tok.Type = Lsquare
	case ']':
		tok.Type = Rsquare
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

func isArray(r rune) bool {
	return r == '['
}

func isGroup(r rune) bool {
	return r == '('
}

func isDelim(r rune) bool {
	return r == ']' || r == ')' || r == ',' || r == '.' || isGroup(r) || isArray(r)
}
