package main

import "errors"

// Rotor errors.
var (
	// ErrEmptyRotor is returned by NewRotor if an empty list is passed:
	// the rotor is not created in that case.
	ErrEmptyRotor = errors.New("rotor: empty list")

	// ErrNoRotationsLeft is returned by Rotate if the counter of possible
	// rotations (the initial list length) has reached 0.
	ErrNoRotationsLeft = errors.New("rotor: no rotations left")
)

// Rotor is a rotating list of strings. The current value is always the
// element at position 0. The number of possible rotations equals the
// length of the list passed at creation time.
type Rotor struct {
	values []string // working copy; values[0] is the current value
	turns  int      // how many rotations are left
}

// NewRotor creates a Rotor from the given list. The list length is
// computed as the counter of possible rotations. An empty list is an
// error: the rotor is not created. The list is copied, so further
// rotations do not affect the caller's slice.
func NewRotor(values []string) (*Rotor, error) {
	if len(values) == 0 {
		return nil, ErrEmptyRotor
	}
	values = append([]string(nil), values...)
	return &Rotor{
		values: values,
		turns:  len(values),
	}, nil
}

// String returns the current (position 0) value. It has the fmt.Stringer
// signature, so a Rotor can be implicitly converted to a string when
// passed to fmt functions or to a function taking a string.
func (r *Rotor) String() string {
	if len(r.values) == 0 {
		return ""
	}
	return r.values[0]
}

// Value is the same as String.
func (r *Rotor) Value() string {
	return r.String()
}

// Rotate decreases the counter of possible rotations. If it becomes 0,
// nothing is moved and ErrNoRotationsLeft is returned. Otherwise the
// first element is moved to the end, so the next element becomes the
// current one (position 0).
func (r *Rotor) Rotate() error {
	if r.turns == 0 {
		return ErrNoRotationsLeft
	}
	r.turns--
	if r.turns == 0 {
		return ErrNoRotationsLeft
	}
	r.moveFirstToLast()
	return nil
}

// Order returns a copy of the list in its current order. If rotations
// have been performed it differs from the initially passed list.
func (r *Rotor) Order() []string {
	return append([]string(nil), r.values...)
}

// moveFirstToLast shifts the first element to the end of the list.
func (r *Rotor) moveFirstToLast() {
	first := r.values[0]
	for i := 0; i+1 < len(r.values); i++ {
		r.values[i] = r.values[i+1]
	}
	r.values[len(r.values)-1] = first
}
