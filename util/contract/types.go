package contract

import "strings"

// Path defines a how to access a field in an Unstructured object.
type Path []string

// Append a field name to a path.
func (p Path) Append(k string) Path {
	return append(p, k)
}

// IsParentOf check if one path is Parent of the other.
func (p Path) IsParentOf(other Path) bool {
	if len(p) >= len(other) {
		return false
	}
	for i := range p {
		if p[i] != other[i] {
			return false
		}
	}
	return true
}

// Equal check if two path are equal (exact match).
func (p Path) Equal(other Path) bool {
	if len(p) != len(other) {
		return false
	}
	for i := range p {
		if p[i] != other[i] {
			return false
		}
	}
	return true
}

// Overlaps return true if two paths are Equal or one IsParentOf the other.
func (p Path) Overlaps(other Path) bool {
	return other.Equal(p) || other.IsParentOf(p) || p.IsParentOf(other)
}

// String returns the path as a dotted string.
func (p Path) String() string {
	return strings.Join(p, ".")
}
