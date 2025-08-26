package settings

import (
	"testing"
)

func TestFeatures_Load_VmwareSystemSerialNumber(t *testing.T) {
	tests := []struct {
		name           string
		featureFlag    string // value for FEATURE_VMWARE_SYSTEM_SERIAL_NUMBER
		unsetFeature   bool   // if true, don't set the feature flag env var
		openshiftVer   string // value for OPENSHIFT_VERSION
		unsetVersion   bool   // if true, don't set the version env var
		expectedResult bool   // expected value for VmwareSystemSerialNumber
	}{
		// Feature enabled (default true) with valid versions
		{
			name:           "feature enabled, version >= 4.20.0",
			featureFlag:    "true",
			openshiftVer:   "4.20.0",
			expectedResult: true,
		},
		{
			name:           "feature enabled by default, version >= 4.20.0",
			unsetFeature:   true, // default is true
			openshiftVer:   "4.20.0",
			expectedResult: true,
		},
		{
			name:           "feature enabled, version > 4.20.0",
			featureFlag:    "true",
			openshiftVer:   "4.21.0",
			expectedResult: true,
		},
		{
			name:           "feature enabled, version with sub-version",
			featureFlag:    "true",
			openshiftVer:   "4.20.5-sub-version",
			expectedResult: true,
		},
		{
			name:           "feature enabled, version with pre-release below minimum",
			featureFlag:    "true",
			openshiftVer:   "4.20.0-alpha.1",
			expectedResult: false, // 4.20.0-alpha.1 < 4.20.0 per semver spec
		},
		{
			name:           "feature enabled, pre-release version above minimum",
			featureFlag:    "true",
			openshiftVer:   "4.21.0-alpha.1",
			expectedResult: true, // 4.21.0-alpha.1 > 4.20.0
		},

		// V-prefixed version support
		{
			name:           "feature enabled, v-prefixed version >= 4.20.0",
			featureFlag:    "true",
			openshiftVer:   "v4.20.0",
			expectedResult: true,
		},
		{
			name:           "feature enabled, v-prefixed version > 4.20.0",
			featureFlag:    "true",
			openshiftVer:   "v4.21.5",
			expectedResult: true,
		},
		{
			name:           "feature enabled, v-prefixed version with sub-version",
			featureFlag:    "true",
			openshiftVer:   "v4.20.5-sub-version",
			expectedResult: true,
		},
		{
			name:           "feature enabled, v-prefixed pre-release above minimum",
			featureFlag:    "true",
			openshiftVer:   "v4.21.0-beta.1",
			expectedResult: true,
		},
		{
			name:           "feature enabled, v-prefixed version < 4.20.0",
			featureFlag:    "true",
			openshiftVer:   "v4.19.9",
			expectedResult: false,
		},

		// Feature enabled but version too low
		{
			name:           "feature enabled, version < 4.20.0",
			featureFlag:    "true",
			openshiftVer:   "4.19.9",
			expectedResult: false,
		},
		{
			name:           "feature enabled, version much lower",
			featureFlag:    "true",
			openshiftVer:   "4.15.0",
			expectedResult: false,
		},
		{
			name:           "feature enabled, pre-release version < 4.20.0",
			featureFlag:    "true",
			openshiftVer:   "4.19.9-beta.1",
			expectedResult: false,
		},

		// Feature explicitly disabled
		{
			name:           "feature disabled, version >= 4.20.0",
			featureFlag:    "false",
			openshiftVer:   "4.20.0",
			expectedResult: false,
		},
		{
			name:           "feature disabled, version < 4.20.0",
			featureFlag:    "false",
			openshiftVer:   "4.19.0",
			expectedResult: false,
		},

		// Invalid or missing version scenarios
		{
			name:           "feature enabled, no version set",
			featureFlag:    "true",
			unsetVersion:   true,
			expectedResult: false,
		},
		{
			name:           "feature enabled, empty version",
			featureFlag:    "true",
			openshiftVer:   "",
			expectedResult: false,
		},
		{
			name:           "feature enabled, invalid version",
			featureFlag:    "true",
			openshiftVer:   "invalid-version",
			expectedResult: false,
		},
		{
			name:           "feature enabled, malformed version",
			featureFlag:    "true",
			openshiftVer:   "4.20.x",
			expectedResult: false,
		},

		// Edge cases
		{
			name:           "feature enabled, exactly minimum version",
			featureFlag:    "true",
			openshiftVer:   "4.20.0",
			expectedResult: true,
		},
		{
			name:           "feature enabled, version just below minimum",
			featureFlag:    "true",
			openshiftVer:   "4.19.99",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			if tt.unsetFeature {
				t.Setenv(FeatureVmwareSystemSerialNumber, "")
				// Note: using empty string instead of os.Unsetenv because t.Setenv
				// automatically unsets after test, and getEnvBool handles empty as unset
			} else {
				t.Setenv(FeatureVmwareSystemSerialNumber, tt.featureFlag)
			}

			if tt.unsetVersion {
				t.Setenv(OpenShiftVersion, "")
			} else {
				t.Setenv(OpenShiftVersion, tt.openshiftVer)
			}

			// Load features and check result
			var features Features
			err := features.Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if features.VmwareSystemSerialNumber != tt.expectedResult {
				t.Errorf("VmwareSystemSerialNumber = %v, want %v",
					features.VmwareSystemSerialNumber, tt.expectedResult)
			}
		})
	}
}

func TestFeatures_isOpenShiftVersionAboveMinimum(t *testing.T) {
	tests := []struct {
		name       string
		ocpVersion string
		minVersion string
		unsetOCP   bool
		expected   bool
	}{
		// Basic version comparisons
		{
			name:       "version equals minimum",
			ocpVersion: "4.20.0",
			minVersion: "4.20.0",
			expected:   true,
		},
		{
			name:       "version above minimum",
			ocpVersion: "4.21.0",
			minVersion: "4.20.0",
			expected:   true,
		},
		{
			name:       "version below minimum",
			ocpVersion: "4.19.0",
			minVersion: "4.20.0",
			expected:   false,
		},

		// Sub-version and pre-release support
		{
			name:       "version with sub-version above minimum",
			ocpVersion: "4.20.5-sub-version",
			minVersion: "4.20.0",
			expected:   true,
		},
		{
			name:       "version with pre-release above minimum",
			ocpVersion: "4.20.1-alpha.1",
			minVersion: "4.20.0",
			expected:   true,
		},
		{
			name:       "pre-release version below minimum",
			ocpVersion: "4.19.9-beta.1",
			minVersion: "4.20.0",
			expected:   false,
		},

		// V-prefixed version support
		{
			name:       "v-prefixed version equals minimum",
			ocpVersion: "v4.20.0",
			minVersion: "4.20.0",
			expected:   true,
		},
		{
			name:       "v-prefixed version above minimum",
			ocpVersion: "v4.21.2",
			minVersion: "4.20.0",
			expected:   true,
		},
		{
			name:       "v-prefixed version below minimum",
			ocpVersion: "v4.19.8",
			minVersion: "4.20.0",
			expected:   false,
		},
		{
			name:       "v-prefixed version with sub-version",
			ocpVersion: "v4.20.3-sub-version",
			minVersion: "4.20.0",
			expected:   true,
		},
		{
			name:       "v-prefixed pre-release above minimum",
			ocpVersion: "v4.20.1-rc.1",
			minVersion: "4.20.0",
			expected:   true,
		},
		{
			name:       "comparing v-prefixed to v-prefixed minimum",
			ocpVersion: "v4.21.0",
			minVersion: "v4.20.0",
			expected:   true,
		},

		// Error cases
		{
			name:       "no OCP version set",
			unsetOCP:   true,
			minVersion: "4.20.0",
			expected:   false,
		},
		{
			name:       "empty OCP version",
			ocpVersion: "",
			minVersion: "4.20.0",
			expected:   false,
		},
		{
			name:       "invalid OCP version",
			ocpVersion: "invalid",
			minVersion: "4.20.0",
			expected:   false,
		},
		{
			name:       "invalid minimum version",
			ocpVersion: "4.20.0",
			minVersion: "invalid",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.unsetOCP {
				t.Setenv(OpenShiftVersion, "")
			} else {
				t.Setenv(OpenShiftVersion, tt.ocpVersion)
			}

			var features Features
			result := features.isOpenShiftVersionAboveMinimum(tt.minVersion)

			if result != tt.expected {
				t.Errorf("isOpenShiftVersionAboveMinimum() = %v, want %v", result, tt.expected)
			}
		})
	}
}
