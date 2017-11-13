package engine

type HollowGroupRequest struct {
	AutoScalingGroupName string `json:"autoScalingGroupName" binding:"required"`
}

func CreateHollowGroup(group HollowGroupRequest) (string, error) {
	return group.AutoScalingGroupName, nil
}
