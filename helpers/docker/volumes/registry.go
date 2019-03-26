package volumes

type registry interface {
	Append(elements ...string)
	Elements() []string
}

type defaultRegistry struct {
	elements []string
}

func (r *defaultRegistry) Append(elements ...string) {
	r.elements = append(r.elements, elements...)
}

func (r *defaultRegistry) Elements() []string {
	return r.elements
}
