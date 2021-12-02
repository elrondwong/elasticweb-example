package controllers

import elasticwebv1 "github.com/cloud-native/elasticweb/pkg/api/v1"

func getExpectReplicas(elasticWeb *elasticwebv1.ElasticWeb) int32 {
	singlePodQPS := *(elasticWeb.Spec.SinglePodQPS)
	totalQPS := *(elasticWeb.Spec.TotalQPS)
	replicas := totalQPS / singlePodQPS
	if totalQPS%singlePodQPS > 0 {
		replicas++
	}
	return replicas
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
