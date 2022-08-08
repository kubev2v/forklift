package model

import liberr "github.com/konveyor/controller/pkg/error"

//
// FK relation.
type FkRelation struct {
	// Data model.
	dm *DataModel
	// Cached sorted definitions.
	sorted Definitions
}

//
// Build constraint DDL.
func (r *FkRelation) DDL(md *Definition) (ddl []string, err error) {
	for _, field := range md.Fields {
		fk := field.Fk()
		if fk == nil || !fk.Must {
			continue
		}
		md, found := r.dm.Find(fk.Table)
		if !found {
			err = liberr.New(
				"FK ref not found.",
				"kind",
				md.Kind,
				"ref",
				fk.Table)
			return
		}
		pk := md.PkField()
		fk.Field = pk.Name
		ddl = append(ddl, fk.DDL(field))
	}

	return
}

//
// Find model definitions that
// reference the specified definition.
func (r *FkRelation) Referencing(md *Definition) (list []*FkRef) {
	list = []*FkRef{}
	for _, refMd := range r.Definitions() {
		for _, field := range refMd.Fields {
			fk := field.Fk()
			if fk == nil {
				continue
			}
			if !md.IsKind(fk.Table) {
				continue
			}
			list = append(
				list,
				&FkRef{
					field:   field.Name,
					cascade: fk.Cascade,
					md:      refMd,
				})
		}
	}

	return
}

//
// Model definitions dependency-sorted as needed to be created.
func (r *FkRelation) Definitions() (list Definitions) {
	r.sort()
	list = r.sorted
	return
}

//
// Sort definitions as needed for creation.
func (r *FkRelation) sort() {
	if r.sorted != nil {
		return
	}
	stack := Definitions{}
	in := r.dm.Definitions()
	for {
		if len(stack) == 0 {
			if len(in) > 0 {
				next := in.Head(true)
				stack.Push(next)
			} else {
				break
			}
		}
		md := stack.Top()
		refMd, found := r.nextRef(&in, md)
		if found {
			stack.Push(refMd)
		} else {
			r.sorted = append(
				r.sorted,
				stack.Pop())
		}
	}
	r.sorted.Reverse()
	return
}

//
// Find next model definition that references the
// specified definition. The found definition is removed
// from the candidate (in) list.
func (r *FkRelation) nextRef(in *Definitions, md *Definition) (ref *Definition, found bool) {
	for i, refMd := range *in {
		for _, field := range refMd.Fields {
			fk := field.Fk()
			if fk == nil || !md.IsKind(fk.Table) {
				continue
			}
			in.Delete(i)
			found = true
			ref = refMd
			return
		}
	}

	return
}

//
// FK reference.
type FkRef struct {
	// Field name.
	field string
	// Cascade
	cascade bool
	// Model definition.
	md *Definition
}
