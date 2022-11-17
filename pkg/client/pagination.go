package client

import (
	"strconv"
)

const (
	MaxPageSize = 100
	MinPageSize = 10
)

type Pager struct {
	Token string
	Size  int
}

// Parse returns the offset and page size.
func (p *Pager) Parse() (int, int, error) {
	var parsedPageSize int
	var offset int
	var err error
	switch {
	case p.Size <= MinPageSize:
		parsedPageSize = MinPageSize

	case p.Size > MaxPageSize:
		parsedPageSize = MaxPageSize

	default:
		parsedPageSize = p.Size
	}

	if p.Token != "" {
		offset, err = strconv.Atoi(p.Token)
		if err != nil {
			return 0, 0, err
		}
	}

	return offset, parsedPageSize, nil
}
