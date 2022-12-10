package query

import (
	"fmt"
)

type MalformedError struct {
	Position
	File    string
	Message string
}

func (e MalformedError) Error() string {
	return fmt.Sprintf("%s %s: %s", e.Position, e.File, e.Message)
}

func invalidQueryForType(kind string) error {
	return fmt.Errorf("given query can not be used with JSON %s", kind)
}
