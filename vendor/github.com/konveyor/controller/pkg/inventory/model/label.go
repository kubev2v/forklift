package model

//
// Labels collection.
type Labels map[string]string

//
// Label model
type Label struct {
	PK     string `sql:"pk"`
	Parent string `sql:"key"`
	Kind   string `sql:"key"`
	Name   string `sql:"key"`
	Value  string `sql:""`
}

func (l *Label) Pk() string {
	return l.PK
}

func (l *Label) String() string {
	return ""
}

func (l *Label) Equals(other Model) bool {
	if label, cast := other.(*Label); cast {
		return label.Kind == l.Kind &&
			label.Parent == l.Parent &&
			label.Name == l.Name &&
			label.Value == l.Value

	}

	return false
}

func (l *Label) Labels() Labels {
	return nil
}
