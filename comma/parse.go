package comma

import (
	"fmt"
	"unicode/utf8"
	"strings"
)

type Parser struct {
	scan *Scanner
	curr Token
	peek Token

	fields []string
}

func Parse(str string, fields []string) (Query, error) {
	p := Parser{
		scan: Scan(strings.TrimSpace(str)),
		fields: fields,
	}
	p.next()
	p.next()
	return p.Parse()
}

func (p *Parser) Parse() (Query, error) {
	switch p.curr.Type {
	case Lcurly:
		return p.parseObject()
	case Lsquare:
		return p.parseArray()
	default:
		return nil, p.parseError("expected '{' or '['")
	}
}

func (p *Parser) parseIndex() (Query, error) {
	if p.is(Literal) || p.is(Number) {
		return p.parseLiteral()
	}
	if err := p.expect(Index, "expected index"); err != nil {
		return nil, err
	}
	return nil, nil
}

func (p *Parser) parseLiteral() (Query, error) {
	return nil, nil
}

func (p *Parser) parseObject() (Query, error) {
	p.next()
	var obj object
	obj.fields = make(map[string]Query)
	for !p.done() && !p.is(Rcurly) {
		if err := p.expect(Literal, "expected literal"); err != nil {
			return nil, err
		}
		if err := p.expect(Colon, "expected ':'"); err != nil {
			return nil, err
		}
		p.next()
		_, err := p.parseIndex()
		if err != nil {
			return nil, err
		}
		switch p.curr.Type {
		case Comma:
			p.next()
			if p.is(Rcurly) {
				return nil, p.parseError("expected key after comma, not '}")
			}
		case Rcurly:
		default:
			return nil, p.parseError("expected ',' or '}")
		}
	}
	if err := p.expect(Rcurly, "expected '}"); err != nil {
		return nil, err
	}
	p.next()
	return &obj, nil
}

func (p *Parser) parseArray() (Query, error) {
	p.next()
	var arr array
	for !p.done() && !p.is(Rsquare) {
		ix, err := p.parseIndex()
		if err != nil {
			return nil, err
		}
		switch p.curr.Type {
		case Comma:
			p.next()
			if p.is(Rsquare) {
				return nil, p.parseError("expected key after comma, not ']")
			}
		case Rsquare:
		default:
			return nil, p.parseError("expected ',' or ']")
		}
	}
	if err := p.expect(Rsquare, "expected ']"); err != nil {
		return nil, err
	}
	p.next()
	return nil, nil
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

const (
	Eof = -(1+iota)
	Literal
	Number
	Index
	Comma
	Lsquare
	Rsquare
	Lcurly
	Rcurly
	Colon
	Invalid
)

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
	case isQuote(s.char):
	case isDigit(s.char):
	case isDelim(s.char):
	case isIndex(s.char):
	case isBlank(s.char):
		s.skipBlank()
		return s.Scan()
	default:
	}
	return tok
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

func isIndex(r rune) bool {
	return r == '$'
}

func isDelim(r rune) bool {
	return isPunct(r) || isGroup(r)
}

func isPunct(r rune) bool {
	return r == ',' || r == ':'
}

func isGroup(r rune) bool {
	return r == '[' || r == ']' || r == '{' || r == '}'
}