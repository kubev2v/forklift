package inspection

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"unicode"

	"github.com/sirupsen/logrus"
)

const (
	defaultLUKSKeyDir = "/etc/luks"
	EnvNBDEClevis     = "V2V_NBDE_CLEVIS"
)

type diskUnlockMethod int

const (
	unlockNone     diskUnlockMethod = iota
	unlockClevis                    // V2V_NBDE_CLEVIS is set
	unlockKeyFiles                  // numeric files found under /etc/luks
)

type diskUnlockInfo struct {
	method diskUnlockMethod
	keys   []string
}

// isLUKSKeyName returns true only for purely numeric names (0, 1, 2, ...).
// This excludes Kubernetes projected-volume bookkeeping files like ..data.
func isLUKSKeyName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// findLUKSKeyFiles returns sorted paths of numeric key files found in dir.
// Returns nil silently if the directory is absent.
func findLUKSKeyFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var keys []string
	for _, e := range entries {
		if !e.IsDir() && isLUKSKeyName(e.Name()) {
			keys = append(keys, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(keys)
	return keys
}

// luksKeyArgs converts key file paths into --key flag pairs.
func luksKeyArgs(keyFiles []string) []string {
	var args []string
	for _, f := range keyFiles {
		args = append(args, "--key", fmt.Sprintf("all:file:%s", f))
	}
	return args
}

// resolveDiskUnlock decides which unlock method to use.
// clevis (V2V_NBDE_CLEVIS set) takes priority over LUKS key files.
func resolveDiskUnlock(logger *logrus.Logger) diskUnlockInfo {
	if os.Getenv(EnvNBDEClevis) != "" {
		if logger != nil {
			logger.Info("clevis/NBDE disk unlock enabled via environment variable")
		}
		return diskUnlockInfo{method: unlockClevis}
	}

	keys := findLUKSKeyFiles(defaultLUKSKeyDir)
	if len(keys) > 0 {
		if logger != nil {
			logger.WithFields(logrus.Fields{
				"count": len(keys),
				"dir":   defaultLUKSKeyDir,
			}).Info("LUKS key files found")
			for _, k := range keys {
				logger.WithField("path", k).Debug("Adding LUKS key file")
			}
		}
		return diskUnlockInfo{method: unlockKeyFiles, keys: keys}
	}

	if logger != nil {
		logger.Debug("No disk unlock method configured (V2V_NBDE_CLEVIS not set, no LUKS key files found)")
	}
	return diskUnlockInfo{method: unlockNone}
}

// Args returns the --key flags for the chosen unlock method.
func (d diskUnlockInfo) Args() []string {
	switch d.method {
	case unlockClevis:
		return []string{"--key", "all:clevis"}
	case unlockKeyFiles:
		return luksKeyArgs(d.keys)
	default:
		return nil
	}
}
