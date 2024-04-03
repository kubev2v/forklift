package vsphere

import (
	"crypto/md5"
	"encoding/hex"
	"net/url"
)

func sanitize(datastoreId string) (sanitizedId string, changed bool) {
	sanitizedId = url.PathEscape(datastoreId)
	if sanitizedId != datastoreId {
		sum := md5.Sum([]byte(datastoreId))
		sanitizedId = hex.EncodeToString(sum[:])
		changed = true
	}
	return
}
