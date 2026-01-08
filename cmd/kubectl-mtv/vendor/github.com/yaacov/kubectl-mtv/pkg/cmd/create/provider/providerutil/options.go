package providerutil

// ProviderOptions contains the options for creating a provider
type ProviderOptions struct {
	Name            string
	Namespace       string
	Secret          string
	URL             string
	Username        string
	Password        string
	CACert          string
	InsecureSkipTLS bool
	// VSphere specific options
	VddkInitImage          string
	SdkEndpoint            string
	UseVddkAioOptimization bool
	VddkBufSizeIn64K       int
	VddkBufCount           int
	// OpenShift specific options
	Token string
	// OpenStack specific options
	DomainName  string
	ProjectName string
	RegionName  string
	// EC2 specific options
	EC2Region             string
	EC2TargetRegion       string
	EC2TargetAZ           string
	EC2TargetAccessKeyID  string // Target account access key (cross-account migrations)
	EC2TargetSecretKey    string // Target account secret key (cross-account migrations)
	AutoTargetCredentials bool   // Auto-fetch target credentials from cluster
}
