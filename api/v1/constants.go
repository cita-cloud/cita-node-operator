/*
Copyright Rivtower Technologies LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

const DefaultImage = "citacloud/cita-node-job:v0.0.2"
