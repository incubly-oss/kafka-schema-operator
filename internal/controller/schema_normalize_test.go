package controller

import (
	_ "embed"
	"regexp"
	"testing"
)

//go:embed testdata/avro_example.json
var avroExample string

//go:embed testdata/avro_example_canonical.json
var avroExampleCanonical string

func TestNormalizeAvroSchema(t *testing.T) {
	reg, _ := regexp.Compile(`\s`)
	expected := reg.ReplaceAllString(avroExampleCanonical, "")
	normalized, err := normalizeAvroSchema(avroExample)
	if err != nil {
		t.Errorf("Error normalizing avro schema: %s", err)
	}
	if normalized != expected {
		t.Errorf("Schema not normlaized properly\nexpected:\t%s\nactual:\t\t%s", expected, normalized)
	}
}
