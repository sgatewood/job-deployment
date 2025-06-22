package status

type JobDeploymentStatus string

const (
	CreatingChild JobDeploymentStatus = "CREATING_CHILD"
	DeletingChild JobDeploymentStatus = "DELETING_CHILD"
	OK            JobDeploymentStatus = "OK"
)
