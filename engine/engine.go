package engine

import (
	"github.com/banzaicloud/hollowtrees/conf"
	"github.com/sirupsen/logrus"
)

var log *logrus.Entry

func init() {
	log = conf.Logger().WithField("package", "engine")
}

type HollowGroupRequest struct {
	AutoScalingGroupName string `json:"autoScalingGroupName" binding:"required"`
}

type CloudEngine interface {
	CreateHollowGroup(group *HollowGroupRequest) (string, error)
}
