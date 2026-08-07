package constants

type DeploymentMode string

const (
	DeploymentModeProduction DeploymentMode = "production"
	DeploymentModeStaging    DeploymentMode = "staging"
)

var Mode DeploymentMode = DeploymentModeProduction

const (
	productionBaseURL = "https://wallet-prodv12.hinkal.io"
	stagingBaseURL    = "https://wallet-staging-v2.hinkal.io"

	snapshotServerEndpoint = "/snapshot-server"
	serverEndpoint         = "/server"
	relayerEndpoint        = "/relayer"

	productionEnclaveURL = "https://enclave-server.hinkal.io"
	stagingEnclaveURL    = "https://enclave-staging-v2.hinkal.io"
)

func baseURL() string {
	if Mode == DeploymentModeStaging {
		return stagingBaseURL
	}
	return productionBaseURL
}

func GetBackEndURL() string { return baseURL() }

func GetSnapshotServerURL() string { return baseURL() + snapshotServerEndpoint }
func GetServerURL() string         { return baseURL() + serverEndpoint }
func GetRelayerURL() string        { return baseURL() + relayerEndpoint }

func GetEnclaveURL() string {
	if Mode == DeploymentModeStaging {
		return stagingEnclaveURL
	}
	return productionEnclaveURL
}
