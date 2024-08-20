package controller

import (
	"fmt"
	"os"
	"strconv"

	"github.com/hamba/avro/v2"
	"incubly.oss/kafka-schema-operator/api/v1beta1"
)

func normalizeAvroSchema(srcSchema string) (string, error) {
	normalized, err := avro.ParseWithCache(srcSchema, "", &avro.SchemaCache{})
	if err != nil {
		return "", err
	}
	return normalized.String(), nil
}

func getenvBool(key string) (bool, error) {
	strEnv := os.Getenv(key)
	if len(strEnv) == 0 {
		return false, nil
	}
	val, err := strconv.ParseBool(strEnv)

	if err != nil {
		return false, fmt.Errorf("unable to parse %s env variable to bool", key)
	}
	return val, nil
}

func maybeNormalizeAvroSchema(srcSchema string, resourceLevelNormalize bool) (string, error) {
	if resourceLevelNormalize {
		return normalizeAvroSchema(srcSchema)
	} else {
		defaultNormalize, err := getenvBool("DEFAULT_NORMALIZE")
		if err != nil {
			return "", err
		}
		if defaultNormalize {
			return normalizeAvroSchema(srcSchema)
		} else {
			return srcSchema, nil
		}
	}
}

func GetMaybeNormalizedSchema(schemaData v1beta1.KafkaSchemaData) (string, error) {
	switch schemaData.Format {
	case v1beta1.AVRO:
		return maybeNormalizeAvroSchema(schemaData.Schema, schemaData.Normalize)
	default:
		return schemaData.Schema, nil
	}
}
