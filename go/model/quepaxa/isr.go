package quepaxa

// Interval Summary Register (ISR)
type ISR[P Proposal[P]] struct {
	t Time // current logical time step
	f P    // first value seen in this step
	a P    // aggregated values so far in this step
	l P    // aggregated values seen in last step
}

func (r *ISR[P]) Record(t Time, p P) (Time, P, P) {

	if r.t.LT(t) {
		// Our recorder state needs to catch up to time t
		if t.s == r.t.s+1 {
			r.l = r.a
		} else {
			r.l = r.l.Nil()
		}
		r.t = t
		r.f = p
		r.a = p

	} else if !t.LT(r.t) {

		// At exactly the right time step - just aggregate proposals
		r.a = r.a.Best(p)

	} else {
		// proposal p is obsolete - just discard it
	}

	// In any case, return the latest recorder state
	return r.t, r.f, r.l
}
