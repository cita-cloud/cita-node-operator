package controllers

import (
	citacloudv1 "github.com/cita-cloud/cita-node-operator/api/v1"
	"sync"
)

var NodeMap sync.Map

type NodeValue struct {
	Status citacloudv1.Status
}

func NewNodeValue(status citacloudv1.Status) *NodeValue {
	return &NodeValue{Status: status}
}
