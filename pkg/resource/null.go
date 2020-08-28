package resource

import "fmt"

// NullResource just captures type and ID.
//
// All other methods panic if being used.
type NullResource struct {
	id  ID
	typ Type
}

// NewNullResource builds a NullResource.
func NewNullResource(id ID, typ Type) *NullResource {
	return &NullResource{
		id:  id,
		typ: typ,
	}
}

// ID implements Resource interface.
func (r *NullResource) ID() ID {
	return r.id
}

// Type implements Resource interface.
func (r *NullResource) Type() Type {
	return r.typ
}

// Version implements Resource interface.
func (r *NullResource) Version() Version {
	panic("not implemented for NullResource")
}

// Spec implements Resource interface.
func (r *NullResource) Spec() interface{} {
	panic("not implemented for NullResource")
}

// Copy implements Resource interface.
func (r *NullResource) Copy() Resource {
	panic("not implemented for NullResource")
}

// String implements Resource interface.
func (r *NullResource) String() string {
	return fmt.Sprintf("NullResource(%s/%s)", r.typ, r.id)
}
