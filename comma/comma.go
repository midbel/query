package comma

import (
	"encoding/csv"
	"errors"
	"io"
)

type Indexer interface {
	Index([]string) error
}

func Convert(r io.Reader, query string) error {
	q, err := Parse(query)
	if err != nil {
		return err
	}
	rs := csv.NewReader(r)

	for {
		row, err := rs.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if err := q.Index(row); err != nil {
			return err
		}
	}
	return nil
}
