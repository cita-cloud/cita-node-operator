package controllers

import "fmt"

// GetNodePortServiceName get node's clusterIP service name
func GetNodePortServiceName(nodeName string) string {
	return fmt.Sprintf("%s-nodeport", nodeName)
}

// GetLogConfigName get node's log config configmap name
func GetLogConfigName(nodeName string) string {
	return fmt.Sprintf("%s-log", nodeName)
}
