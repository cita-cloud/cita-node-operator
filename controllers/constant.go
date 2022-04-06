package controllers

const (
	NetworkContainer    = "network"
	ConsensusContainer  = "consensus"
	ExecutorContainer   = "executor"
	StorageContainer    = "storage"
	ControllerContainer = "controller"
	KmsContainer        = "kms"

	AccountVolumeName         = "account"
	KmsDBMountPath            = "/mnt"
	LogConfigVolumeName       = "log-config"
	NodeConfigVolumeName      = "node-config"
	NodeConfigVolumeMountPath = "/etc/cita-cloud/config"
	DataVolumeName            = "datadir"
	DataVolumeMountPath       = "/data"
	LogConfigVolumeMountPath  = "/etc/cita-cloud/log"
	LogDir                    = DataVolumeMountPath + "/logs"

	NodeConfigFile          = "config.toml"
	ControllerLogConfigFile = "controller-log4rs.yaml"
	ExecutorLogConfigFile   = "executor-log4rs.yaml"
	KmsLogConfigFile        = "kms-log4rs.yaml"
	NetworkLogConfigFile    = "network-log4rs.yaml"
	StorageLogConfigFile    = "storage-log4rs.yaml"
	ConsensusLogConfigFile  = "consensus-log4rs.yaml"

	NetworkPort    = 40000
	ControllerPort = 50004
	ExecutorPort   = 50002

	CaCert = "cert.pem"
	CaKey  = "key.pem"

	NodeCert = "cert.pem"
	NodeCsr  = "csr.pem"
	NodeKey  = "key.pem"
)
