package inventory

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"log"

	"github.com/kubev2v/forklift/pkg/lib/gob"

	"github.com/google/uuid"
)

type UUIDMap struct {
	m map[string]string
}

func NewUUIDMap() *UUIDMap {
	return &UUIDMap{
		m: make(map[string]string),
	}
}

func (um *UUIDMap) GetUUID(object interface{}, key string) string {
	var id string
	id, ok := um.m[key]

	if !ok {
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)

		if err := enc.Encode(object); err != nil {
			log.Fatal(err)
		}

		hash := sha256.Sum256(buf.Bytes())
		id = hex.EncodeToString(hash[:])
		if len(id) > 36 {
			id = id[:36]
		}
		um.m[key] = id
	}
	return id
}

func isValidUUID(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}
