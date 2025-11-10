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
}
