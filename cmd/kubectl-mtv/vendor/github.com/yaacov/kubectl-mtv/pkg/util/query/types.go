package query

// QueryOptions contains the parsed query string options
type QueryOptions struct {
	Select     []SelectOption
	Where      string
	OrderBy    []OrderOption
	Limit      int
	HasSelect  bool
	HasOrderBy bool
	HasLimit   bool
}

// SelectOption represents a column to be selected and its optional alias
type SelectOption struct {
	Field   string
	Alias   string
	Reducer string // one of "sum","len","any","all", or empty
}

// OrderOption represents a field to sort by and its direction
type OrderOption struct {
	Field      SelectOption
	Descending bool
}
