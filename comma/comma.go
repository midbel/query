package comma

import (
	"bufio"
	"encoding/csv"
	"errors"
	"io"
	"strings"
)

type Converter struct {
	Fields []string
	SkipHeader bool
	delim  rune
}

func Csv() *Converter {
	return createConverter(',')
}

func Tsv() *Converter {
	return createConverter('\t')
}

func Space() *Converter {
	return createConverter(' ')
}

func ConvertToString(r io.Reader, query string) (string, error) {
	var str strings.Builder
	if err := Csv().Convert(r, &str, query); err != nil {
		return "", err
	}
	return str.String(), nil
}

func createConverter(comma rune) *Converter {
	return &Converter{
		delim: comma,
	}
}

func (c Converter) Convert(r io.Reader, w io.Writer, query string) error {
	q, err := Parse(query)
	if err != nil {
		return err
	}
	var (
		rs = csv.NewReader(r)
		ws = bufio.NewWriter(w)
	)
	rs.TrimLeadingSpace = true
	rs.Comma = c.delim

	if c.SkipHeader {
		rs.Read()
	}
	ws.WriteRune('[')

	for i := 0; ; i++ {
		row, err := rs.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		str, err := q.Index(row)
		if err != nil {
			return err
		}
		if i > 0 {
			ws.WriteRune(',')
			ws.WriteRune(' ')
		}
		ws.WriteString(str)
	}
	ws.WriteRune(']')
	return ws.Flush()

	return nil
}
