package minnet

// Vector timestemp
type vec []int

// Return true if vector timestamp x is causally before or equal to y.
func (x vec) le(y vec) bool {
	for i := range x {
		if x[i] > y[i] {
			return false
		}
	}
	return true
}

// Set z to the elementwise maximum of vectors x and y.
// Inputs x and/or y can be the same as target z.
func (z vec) max(x, y vec) {
	for i := range z {
		if x[i] > y[i] {
			z[i] = x[i]
		} else {
			z[i] = y[i]
		}
	}
}


