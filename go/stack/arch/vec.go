package arch


// Seq represents a node-local event sequence number.
type Seq int64

// Vec represents a vector timestamp with a sequence number per node.
type Vec []Seq

// Mat represents a matrix timestamp with a vector per participant.
type Mat []Vec


// Return a copy of this vector timestamp.
func (x Vec) copy() Vec {
	return append(Vec{}, x...)
}

// Return true if vector timestamp x is causally before or equal to y,
// and false if x is causally after or incomparable to y.
func (x Vec) Le(y Vec) bool {
	for i := range x {
		if x[i] > y[i] {
			return false
		}
	}
	return true
}

// Set z to the elementwise maximum of vector timestamps x and y.
// Inputs x and/or y can be the same as target z.  Returns z.
func (z Vec) Max(x, y Vec) Vec {
	for i := range z {
		if x[i] > y[i] {
			z[i] = x[i]
		} else {
			z[i] = y[i]
		}
	}
	return z
}

//func (v Vec) String()  {
//	fmt.Sprintf("%v", []int(v))
//}

