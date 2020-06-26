package model

type Folder struct {
	Base
	Children string `sql:""`
}

type Datacenter struct {
	Base
	Cluster   string `sql:""`
	Network   string `sql:""`
	Datastore string `sql:""`
	VM        string `sql:""`
}

type Cluster struct {
	Base
	Host string `sql:""`
}

type Host struct {
	Base
	Maintenance string `sql:""`
	VM          string `sql:""`
}

type Network struct {
	Base
	Tag string `sql:""`
}

type Datastore struct {
	Base
	Type        string `sql:""`
	Capacity    int64  `sql:""`
	Free        int64  `sql:""`
	Maintenance string `sql:""`
}

type VM struct {
	Base
}
