package pkg

type ChainDeployMethod string

const (
	PythonOperator ChainDeployMethod = "python"
	Helm           ChainDeployMethod = "helm"
	CRDOperator    ChainDeployMethod = "crd"
)
