package downloader

import (
	"errors"
	"sync"
)

type r struct {
	l    *sync.RWMutex
	max  int
	data []chunk
}

func newR(max int) *r {
	return &r{
		l:    &sync.RWMutex{},
		max:  max,
		data: make([]chunk, 0, max),
	}
}

func (r *r) add(c chunk) error {
	r.l.Lock()
	defer r.l.Unlock()

	if len(r.data) >= r.max {
		return errors.New("out of limit")
	}

	r.data = append(r.data, c)
	return nil
}

func (r *r) getAll() []chunk {
	r.l.RLock()
	defer r.l.RUnlock()

	return r.data
}
