package controller

import (
	"os"

	"incubly.oss/kafka-schema-operator/api/v1beta1"
	"incubly.oss/kafka-schema-operator/internal/schemareg"
)

func performCleanup(resource *v1beta1.KafkaSchema, srClient *schemareg.SrClient) error {
	policy := getCleanupPolicy(resource)
	subjectName := resource.Status.Subject
	switch policy {
	case v1beta1.SOFT:
		return srClient.DeleteSubject(subjectName, false)
	case v1beta1.HARD:
		err := srClient.DeleteSubject(subjectName, false)
		if err != nil {
			return err
		} else {
			return srClient.DeleteSubject(subjectName, true)
		}
	case v1beta1.DISABLED:
	default:
		// do nothing
	}
	return nil
}

func getCleanupPolicy(schema *v1beta1.KafkaSchema) v1beta1.CleanupPolicy {
	resourcePolicy := schema.Spec.CleanupPolicy
	if len(resourcePolicy) > 0 {
		return resourcePolicy
	}
	defaultCleanupPolicy := os.Getenv("DEFAULT_CLEANUP_POLICY")
	if len(defaultCleanupPolicy) > 1 {
		return v1beta1.CleanupPolicy(defaultCleanupPolicy)
	}
	return v1beta1.DISABLED
}
