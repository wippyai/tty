// SPDX-License-Identifier: MPL-2.0

package screen

// ring is a bounded scrollback deque. It grows by append up to limit lines
// and only then starts overwriting the oldest line; start is non-zero only
// while the ring is full.
type ring struct {
	lines []line
	start int
	limit int
}

func (r *ring) len() int { return len(r.lines) }

func (r *ring) at(i int) *line {
	return &r.lines[(r.start+i)%len(r.lines)]
}

// push stores l as the newest line. It returns a line whose storage the
// caller may reuse: the evicted oldest line, l itself when nothing is kept,
// or the zero line when the ring simply grew.
func (r *ring) push(l line) line {
	switch {
	case r.limit <= 0:
		return l
	case len(r.lines) < r.limit:
		r.lines = append(r.lines, l)
		return line{}
	}
	old := r.lines[r.start]
	r.lines[r.start] = l
	r.start = (r.start + 1) % len(r.lines)
	return old
}

// pop removes and returns the newest line.
func (r *ring) pop() line {
	i := (r.start + len(r.lines) - 1) % len(r.lines)
	l := r.lines[i]
	if i == len(r.lines)-1 {
		r.lines[i] = line{}
		r.lines = r.lines[:i]
	} else {
		r.linearize(len(r.lines))
		r.lines[len(r.lines)-1] = line{}
		r.lines = r.lines[:len(r.lines)-1]
	}
	return l
}

// linearize rewrites the ring as its newest keep lines in order, start 0.
func (r *ring) linearize(keep int) {
	n := len(r.lines)
	if keep > n {
		keep = n
	}
	if r.start == 0 && keep == n {
		return
	}
	out := make([]line, keep)
	for i := 0; i < keep; i++ {
		out[i] = *r.at(n - keep + i)
	}
	r.lines, r.start = out, 0
}

func (r *ring) setLimit(limit int) {
	if limit < 0 {
		limit = 0
	}
	r.limit = limit
	if len(r.lines) > limit {
		r.linearize(limit)
		if limit == 0 {
			r.lines = nil
		}
		return
	}
	r.linearize(len(r.lines))
}

func (r *ring) clear() {
	r.lines, r.start = nil, 0
}
