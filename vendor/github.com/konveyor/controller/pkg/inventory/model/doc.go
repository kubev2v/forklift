package model

//
// New database.
func New(path string, models ...interface{}) DB {
	return &Client{
		path:   path,
		models: models,
	}
}
