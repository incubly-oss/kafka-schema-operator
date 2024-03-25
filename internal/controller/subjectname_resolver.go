package controller

import (
	"encoding/json"
	"fmt"
	"github.com/riferrei/srclient"

	"incubly.oss/kafka-schema-operator/api/v1beta1"
)

func resolveSubjectName(spec *v1beta1.KafkaSchemaSpec) (string, error) {
	switch spec.NamingStrategy {
	case v1beta1.TOPIC:
		topicName, err := notBlank(spec.TopicName, "topicName")
		if err != nil {
			return "", err
		} else {
			return topicName + "-value", nil
		}
	case v1beta1.RECORD:
		return extractRecordName(spec.Data)
	case v1beta1.TOPIC_RECORD:
		topicName, err := notBlank(spec.TopicName, "topicName")
		if err != nil {
			return "", err
		}
		recordName, err := extractRecordName(spec.Data)
		if err != nil {
			return "", err
		} else {
			return topicName + "-" + recordName, nil
		}
	case "":
		return notBlank(spec.SubjectName, "subjectName")
	}
	return "", fmt.Errorf("unsupported subject naming strategy %s", spec.NamingStrategy)
}

type AvroSchema struct {
	Package string `json:"package,omitempty"`
	Name    string `json:"name"`
	Type    string `json:"type"`
}

func extractRecordName(data v1beta1.KafkaSchemaData) (string, error) {
	if data.Format != srclient.Avro {
		return "", fmt.Errorf("record name strategy is only supported for AVRO schemas")
	}
	avroSchema := AvroSchema{}
	err := json.Unmarshal([]byte(data.Schema), &avroSchema)
	if err != nil {
		return "", err
	}
	if avroSchema.Type != "record" {
		return "", fmt.Errorf("record name strategy is only supported for records, got type=%s", avroSchema.Type)
	}
	if len(avroSchema.Name) == 0 {
		return "", fmt.Errorf("record Name missing in schema")
	}
	if len(avroSchema.Package) > 0 {
		return avroSchema.Package + "." + avroSchema.Name, nil
	} else {
		return avroSchema.Name, nil
	}
}

func notBlank(s string, msg string) (string, error) {
	if len(s) > 0 {
		return s, nil
	} else {
		return "", fmt.Errorf("%s was required but is blank", msg)
	}
}
