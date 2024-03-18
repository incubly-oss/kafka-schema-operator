package schemareg_mock

import (
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"io"
	"k8s.io/apimachinery/pkg/util/json"
	"kafka-schema-operator/schemareg"
	"net/http"
	"regexp"
	"strings"
)

type Subject struct {
	CompatibilityMode string
	SchemaRefs        map[int]string
}

func (s Subject) assertSchemaCompatibility(policy SchemaCompatibilityPolicy, newSchema string) error {
	if policy != nil && !policy.isCompatible(s.SchemaRefs, newSchema) {
		return errors.New("new schema is not compatible with subject")
	} else {
		return nil
	}
}

type SchemaCompatibilityPolicy interface {
	isCompatible(existingSchemas map[int]string, newSchema string) bool
}

type SchemaRegMock struct {
	Subjects              map[string]*Subject
	Schemas               []string
	compatibilityPolicies map[string]SchemaCompatibilityPolicy
	logger                logr.Logger
}

func NewSchemaRegMock(compatibilityPolicies map[string]SchemaCompatibilityPolicy, logger logr.Logger) *SchemaRegMock {
	return &SchemaRegMock{
		Subjects:              map[string]*Subject{},
		Schemas:               []string{},
		compatibilityPolicies: compatibilityPolicies,
		logger:                logger,
	}
}

func (m *SchemaRegMock) GetServer() *ghttp.Server {
	server := ghttp.NewServer()

	server.RouteToHandler(
		"POST",
		regexp.MustCompile(`^/subjects/[a-zA-Z0-9-_.]+/versions$`),
		m.registerSubject(),
	)
	server.RouteToHandler(
		"DELETE",
		regexp.MustCompile(`^/subjects/[a-zA-Z0-9-_.]+$`),
		m.deleteSubject(),
	)
	server.RouteToHandler(
		"PUT",
		regexp.MustCompile(`^/config/[a-zA-Z0-9-_.]+$`),
		m.setCompatibilityMode(),
	)

	return server
}

func (m *SchemaRegMock) registerSubject() http.HandlerFunc {
	return ghttp.CombineHandlers(
		func(w http.ResponseWriter, req *http.Request) {
			subjectName := strings.Split(req.URL.Path, "/")[2]

			m.logger.Info("Schema-reg: registering subject " + subjectName)

			registerSchemaReq := readJsonBody(req, &schemareg.RegisterSchemaReq{})

			schema, parseSchemaErr := parseSchema(*registerSchemaReq)
			if parseSchemaErr != nil {
				w.WriteHeader(422)
				w.Write([]byte(`{"error_code":42201,"message":"Invalid schema"}`))
				return
			}

			if existingSubject, ok := m.Subjects[subjectName]; ok {
				incompatibleSchemaErr := existingSubject.assertSchemaCompatibility(m.compatibilityPolicies[existingSubject.CompatibilityMode], schema)
				if incompatibleSchemaErr != nil {
					w.WriteHeader(409)
					w.Write([]byte(`{"error_code":409,"message":"Incompatible schema"}`))
					return
				}
			} else {
				m.Subjects[subjectName] = &Subject{
					CompatibilityMode: "BACKWARD",
					SchemaRefs:        map[int]string{},
				}
			}

			schemaId := m.registerSchema(schema)
			m.Subjects[subjectName].SchemaRefs[schemaId] = schema

			w.Write([]byte(fmt.Sprintf(`{"id": %d}`, schemaId)))
			w.WriteHeader(200)
		},
	)
}

func parseSchema(req schemareg.RegisterSchemaReq) (string, error) {
	var js map[string]interface{}
	err := json.Unmarshal([]byte(req.Schema), &js)
	if err != nil {
		return "", err
	} else {
		return req.Schema, nil
	}
}

func readRawBody(req *http.Request) []byte {
	payload, err := io.ReadAll(req.Body)
	Expect(err).Should(Succeed())
	return payload
}

func readJsonBody[T interface{}](req *http.Request, body *T) *T {
	rawBody := readRawBody(req)
	Expect(json.Unmarshal(rawBody, body)).Should(Succeed())
	return body
}

func (m *SchemaRegMock) registerSchema(schema string) int {
	for existingId, existingSchema := range m.Schemas {
		if existingSchema == schema {
			return existingId
		}
	}
	m.Schemas = append(m.Schemas, schema)
	return len(m.Schemas) - 1
}

func (m *SchemaRegMock) deleteSubject() http.HandlerFunc {
	return ghttp.CombineHandlers(
		func(w http.ResponseWriter, req *http.Request) {
			subjectName := strings.Split(req.URL.Path, "/")[2]
			if _, ok := m.Subjects[subjectName]; ok {
				delete(m.Subjects, subjectName)
				w.WriteHeader(200)
				// TODO OK response
			} else {
				w.WriteHeader(500)
				// TODO response if subject doesn't exist?
			}
		},
	)
}

func (m *SchemaRegMock) setCompatibilityMode() http.HandlerFunc {
	return ghttp.CombineHandlers(
		func(w http.ResponseWriter, req *http.Request) {
			subjectName := strings.Split(req.URL.Path, "/")[2]
			compatibilityMode := readJsonBody(req, &schemareg.SetCompatibilityModeReq{})
			if validateCompatibilityMode(compatibilityMode.Compatibility) != nil {
				w.WriteHeader(422)
				w.Write([]byte(`{"error_code":42203,"message":"Invalid compatibility level"}`))
			}
			if existingSubject, ok := m.Subjects[subjectName]; ok {
				existingSubject.CompatibilityMode = compatibilityMode.Compatibility
				w.WriteHeader(200)
				// TODO OK response
			} else {
				w.WriteHeader(500)
				// TODO response if subject doesn't exist?
			}
		},
	)
}

func (m *SchemaRegMock) Clear() {
	m.logger.Info("Removing previously registered subjects and schemas")
	m.Subjects = map[string]*Subject{}
	m.Schemas = []string{}
}

func validateCompatibilityMode(mode string) error {
	validModes := []string{"BACKWARD", "BACKWARD_TRANSITIVE", "FORWARD", "FORWARD_TRANSITIVE", "FULL", "FULL_TRANSITIVE", "NONE"}
	for _, validMode := range validModes {
		if validMode == mode {
			return nil
		}
	}
	return errors.New(fmt.Sprintf(`Invalid compatibility mode %s`, mode))
}
