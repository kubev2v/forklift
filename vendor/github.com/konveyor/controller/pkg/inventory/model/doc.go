// The `model` package essentially provides a lightweight object
// relational model (ORM) based on sqlite3 intended to support the
// needs of the `container` package.
// Each entity (table) is modeled by a struct.  Each field (column)
// is described using tags:
//   `sql:"pk"`
//       The primary key.
//   `sql:"key"`
//       The field is part of the natural key.
//   `sql:"fk:T(F)"`
//       Foreign key `T` = model type, `F` = model field.
//   `sql:"unique(G)"`
//       Unique index. `G` = unique-together fields.
//   `sql:"const"`
//       The field is immutable and not included on update.
// Each struct must implement the `Model` interface.
// Basic CRUD operations may be performed on each model using
// the `DB` interface which together with the `Model` interface
// provides value-added features and optimizations.
//
// Examples:
//
// Define a model.
//   type Person struct {
//       ID    string `sql:"pk"`
//       First string `sql:"key"`
//       Last  string `sql:"key"
//       Age   int    `sql:""`
//   }
//
//   func (p *Person) Pk() string {...}
//   func (p *Person) Equals(other Model) bool {...}
//   func (p *Person) Labels() {...}
//   func (p *Person) String() string {...}
//
// Insert the model:
//   person := &Person{
//       First: "Elmer",
//       Last:  "Fudd",
//       Age: 55,
//   }
//
//   err := DB.Insert(person)
//
// In the event the primary key (PK) field is not populated,
// the DB will derive (generate) its value as a sha1 of the
// natural key fields.
//
// Update the model:
//   person.Age = 62
//   err := DB.Update(person)
//
// Delete the model by natural key:
//   person := &Person{
//       First: "Elmer",
//       Last:  "Fudd",
//   }
//
//   err := DB.Delete(person)
//
// Get (fetch) a single model by natural key.
// This will populate the fields with data from the DB.
//   person := &Person{
//       First: "Elmer",
//       Last:  "Fudd",
//   }
//
//  err := DB.Get(person)
//
// List (fetch) all models.
//   persons := []Person{}
//   err := DB.List(&persons, ListOptions{})
//
// List (fetch) specific models.
// The `ListOptions` may be used to qualify or paginate the
// List() result set.  All predicates may be combined.
//
// Count (only):
//   err := DB.List(&persons, ListOptions{Count: true})
//
// Paginate the result:
//   err := DB.List(
//       &persons,
//       ListOptions{
//           Page: {
//               Offset: 3, // page 3.
//               Limit: 10, // 10 models per page.
//           },
//       })
//
// List specific models.
// List persons with the last name of "Fudd" and legal to vote.
//   err := DB.List(
//       &persons,
//       ListOptions{
//           Predicate: And(
//               Eq("Name", "Fudd"),
//               Gt("Age": 17),
//           },
//       })
//
package model

//
// New database.
func New(path string, models ...interface{}) DB {
	return &Client{
		path:   path,
		models: models,
	}
}
