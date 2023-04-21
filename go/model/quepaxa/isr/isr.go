package isr

type integer interface {
	~int | ~int32 | ~int64 | ~uint | ~uint32 | ~uint64
}

// Ops is a parameterized interface type
// representing operations on data values that an ISR needs.
type Ops[D any] interface {
	Nil() D			// return the nil value
	Agg(a,b D) D		// aggregate two data values
}

// Interval Summary Register (ISR)
//
type ISR[S integer, D any, O Ops[D]] struct {
	s S			// current logical time step
	f D			// first value seen in this step
	a D			// aggregated values so far in this step
	l D			// aggregated values seen in last step
	o O			// operations on data values
}

func (r *ISR[S,D,O]) Record(s S, v D) (S, D, D) {
	if s == r.s {
		r.a = r.o.Agg(r.a, v)
	} else if s > r.s {
		if s == r.s+1 {
			r.l = r.a
		} else {
			r.l = r.o.Nil()
		}
		r.s = s
		r.f = v
		r.a = v
	}
	return r.s, r.f, r.l
}


// MaxInt provides operations for integer-valued ISRs,
// using zero as the base value and maximum as the aggregator.
type MaxInt[I integer] struct{}

func (_ MaxInt[I]) Nil() I {
	return 0
}

func (_ MaxInt[I]) Agg(a, b I) I {
	if a > b {
		return a
	} else {
		return b
	}
}

var testIsr ISR[int, int, MaxInt[int]]

