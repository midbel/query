package comma

import (
	"io"
	"encoding/csv"
)


type Query interface {
	Select([]string) error
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
		if err := q.Select(row); err != nil {
			return err
		}
	}
	return nil
}

type object struct {
	fields map[string]Query
	keys   []string
}

func (o *object) Select(row []string) error {
	return nil
}

type array struct {
	list []Query
}

func (a *array) Select(row []string) error {
	return nil
}

type literal struct {
	value string
}

func (i *literal) Select([]string) error {
	return nil
} 

type index struct {
	index int
}

func (i *index) Select(row []string) error {
	return nil
}

type list struct {
	list []int
}

func (i *list) Select(row []string) error {
	return nil
}

type interval struct {
	beg int
	end int
}

func (i *interval) Select(row []string) error {
	return nil
}