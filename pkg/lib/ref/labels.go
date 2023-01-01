package ref

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

// Labels
const (
	// = Application
	PartOfLabel = "app.kubernetes.io/part-of"
)

var (
	// Application identifier included in reference labels.
	// **Must set be by the using application.
	Application = ""
)

// Build unique reference label for an object.
// Format: <kind> = <uid>
func Label(object v1.Object) (label, uid string) {
	label = string(object.GetUID())
	uid = strings.ToLower(ToKind(object))
	return
}

// Build reference labels for an object.
// Includes both `Application` and unique labels.
func Labels(object v1.Object) map[string]string {
	label, uid := Label(object)
	return map[string]string{
		PartOfLabel: Application,
		label:       uid,
	}
}
