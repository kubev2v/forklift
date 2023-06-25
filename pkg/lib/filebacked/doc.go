/*
Provides file-backed collections.
*/
package filebacked

import "encoding/gob"

func init() {
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
}
