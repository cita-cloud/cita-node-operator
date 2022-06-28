package chain

import "strings"

func GetNodeList(nodeListInput string) []string {
	if nodeListInput == "*" {
		return []string{"*"}
	}
	return strings.Split(nodeListInput, ",")
}

func AllNode(nodeStr string) bool {
	if nodeStr == "*" {
		return true
	}
	return false
}
