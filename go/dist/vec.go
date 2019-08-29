package dist

// Vector timestemp
type vec []int

// Return a copy of this vector
func (v vec) copy() vec {
	return append(vec{}, v...)
}

// Return true if vector timestamp v is causally before or equal to y.
func (v vec) le(y vec) bool {
	for i := range v {
		if v[i] > y[i] {
			return false
		}
	}
	return true
}

// Set v to the elementwise maximum of vectors x and y.
// Inputs x and/or y can be the same as target v.
func (v vec) max(x, y vec) {
	for i := range v {
		if x[i] > y[i] {
			v[i] = x[i]
		} else {
			v[i] = y[i]
		}
	}
}

//func (v vec) String()  {
//	fmt.Sprintf("%v", []int(v))
//}
