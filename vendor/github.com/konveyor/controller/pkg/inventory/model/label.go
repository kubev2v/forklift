package model

type Labels map[string]string

type Label struct {
	Parent string `sql:"unique(a),key"`
	Kind   string `sql:"unique(a),key"`
	Name   string `sql:"unique(a)"`
	Value  string `sql:""`
}
