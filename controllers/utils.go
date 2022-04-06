package controllers

import "k8s.io/apimachinery/pkg/api/equality"

// IsEqual check two object is equal.
func IsEqual(obj1, obj2 interface{}) bool {
	return equality.Semantic.DeepEqual(obj1, obj2)
}
