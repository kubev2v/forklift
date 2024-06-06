package recordingrules

import "github.com/machadovilaca/operator-observability/pkg/operatorrules"

func Register(namespace string) error {
	return operatorrules.RegisterRecordingRules(
		migrationsRecordingRules,
		plansRecordingRules,
	)
}
