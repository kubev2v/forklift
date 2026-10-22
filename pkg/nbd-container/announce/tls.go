package announce

// Mutual-TLS material file names. The appliance secret uses these as data keys
// so that what is in the secret is what lands under the appliance certs dir.
const (
	CACert     = "ca-cert.pem"
	CAKey      = "ca-key.pem" // kept in-cluster; setup mints per-appliance server leaves
	ServerCert = "server-cert.pem"
	ServerKey  = "server-key.pem"
	ClientCert = "client-cert.pem"
	ClientKey  = "client-key.pem"
)

// SecretTLSKeys is every TLS data key written into the appliance secret.
var SecretTLSKeys = []string{CACert, CAKey, ServerCert, ServerKey, ClientCert, ClientKey}
