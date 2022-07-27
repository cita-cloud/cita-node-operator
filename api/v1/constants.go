package v1

type JobConditionType string

// These are valid conditions of a job.
const (
	// JobActive means the job is active.
	JobActive JobConditionType = "Active"
	// JobComplete means the job has completed its execution.
	JobComplete JobConditionType = "Complete"
	// JobFailed means the job has failed its execution.
	JobFailed JobConditionType = "Failed"
)

const (
	CITANodeJobServiceAccount     = "cita-node-job"
	CITANodeJobClusterRole        = "cita-node-job"
	CITANodeJobClusterRoleBinding = "cita-node-job"
)

const DefaultImage = "registry.devops.rivtower.com/cita-cloud/operator/cita-node-job:v0.0.1"
