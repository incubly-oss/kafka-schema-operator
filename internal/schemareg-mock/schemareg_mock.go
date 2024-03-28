package schemareg_mock

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"incubly.oss/kafka-schema-operator/api/v1beta1"
	"incubly.oss/kafka-schema-operator/internal/schemareg"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"k8s.io/apimachinery/pkg/util/json"
)

type SchemaRef struct {
	version  int
	schemaId int
}

type Subject struct {
	CompatibilityMode v1beta1.CompatibilityMode
	SchemaRefs        []SchemaRef
}

func (s *Subject) setSchemaAsCurrentVersion(schemaId int) {
	if len(s.SchemaRefs) == 0 {
		s.SchemaRefs = append(s.SchemaRefs, SchemaRef{
			version:  0,
			schemaId: schemaId,
		})
	} else {
		currentRef := s.SchemaRefs[len(s.SchemaRefs)-1]
		if currentRef.schemaId != schemaId {
			s.SchemaRefs = append(s.SchemaRefs, SchemaRef{
				version:  currentRef.version + 1,
				schemaId: schemaId,
			})
		}
	}
}

type SchemaRegMock struct {
	Subjects            map[string]*Subject
	Schemas             map[int]string
	SoftDeletedSubjects map[string]*Subject
	HardDeletedSubjects map[string]*Subject
	logger              logr.Logger
	nextSchemaId        int
	injectedErrors      map[InjectOnApi]*InjectedError
}

func NewSchemaRegMock(logger logr.Logger) *SchemaRegMock {
	return &SchemaRegMock{
		Subjects:            map[string]*Subject{},
		Schemas:             map[int]string{},
		SoftDeletedSubjects: map[string]*Subject{},
		HardDeletedSubjects: map[string]*Subject{},
		logger:              logger,
		nextSchemaId:        100,
	}
}

func (m *SchemaRegMock) GetServer() *ghttp.Server {
	server := ghttp.NewServer()

	server.RouteToHandler(
		"POST",
		regexp.MustCompile(`^/subjects/[a-zA-Z0-9-_.]+/versions$`),
		m.registerSubjectHandler(),
	)
	server.RouteToHandler(
		"DELETE",
		regexp.MustCompile(`^/subjects/[a-zA-Z0-9-_.]+$`),
		m.deleteSubjectHandler(),
	)
	server.RouteToHandler(
		"PUT",
		regexp.MustCompile(`^/config/[a-zA-Z0-9-_.]+$`),
		m.setCompatibilityModeHandler(),
	)

	return server
}

func (m *SchemaRegMock) registerSubjectHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if m.handledByErrorsInjector(RegisterSubject, w) {
			return
		}

		subjectName := strings.Split(req.URL.Path, "/")[2]

		m.logger.Info("Schema-reg: registering subject " + subjectName)

		registerSchemaReq := readJsonBody(req, &schemareg.RegisterSchemaReq{})

		schema, parseSchemaErr := parseSchema(*registerSchemaReq)
		if parseSchemaErr != nil {
			w.WriteHeader(422)
			_, _ = w.Write([]byte(`{"error_code":42201,"message":"Invalid schema"}`))
			return
		}

		if _, ok := m.Subjects[subjectName]; !ok {
			m.Subjects[subjectName] = &Subject{
				CompatibilityMode: v1beta1.BACKWARD,
				SchemaRefs:        []SchemaRef{},
			}
		}

		schemaId := m.registerSchema(schema)
		m.Subjects[subjectName].setSchemaAsCurrentVersion(schemaId)

		_, _ = w.Write([]byte(fmt.Sprintf(`{"id": %d}`, schemaId)))
		w.WriteHeader(200)
	}
}

func parseSchema(req schemareg.RegisterSchemaReq) (string, error) {
	if req.SchemaType == v1beta1.AVRO {
		return parseAvroSchema(req.Schema)
	} else {
		// TODO: implement for other schema types if needed. for now they're all valid
		return req.Schema, nil
	}
}

func parseAvroSchema(schema string) (string, error) {
	/*
		According to spec, valid Avro schemas are:
		JSON String, Array, Object or tokens 'null', 'true' or 'false'
		So, basically, anything that you could use as a value under any json key.
		Let's wrap the schema with "fake" json structure and see if it can be unmarshalled
	*/
	var jsMap map[string]interface{}
	wrappedJson := `{"x":` + schema + `}`
	if err := json.Unmarshal([]byte(wrappedJson), &jsMap); err == nil {
		return schema, nil
	} else {
		return "", err
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
	schemaId := m.nextSchemaId
	m.Schemas[schemaId] = schema
	m.nextSchemaId += 1
	return schemaId
}

func (m *SchemaRegMock) deleteSubjectHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if m.handledByErrorsInjector(DeleteSubject, w) {
			return
		}
		subjectName := strings.Split(req.URL.Path, "/")[2]
		queryParams, _ := url.ParseQuery(req.URL.RawQuery)
		permanent := queryParams.Get("permanent") == "true"
		versions, err := m.DeleteSubject(subjectName, permanent)
		if err != nil {
			w.WriteHeader(404)
			_, _ = w.Write([]byte(err.Error()))
		} else {
			w.WriteHeader(200)
			versionsAsStrings := strings.Fields(fmt.Sprint(versions))
			resBody := strings.Trim(strings.Join(versionsAsStrings, ","), "[]")
			_, _ = w.Write([]byte(resBody))
		}
	}
}

func (m *SchemaRegMock) setCompatibilityModeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if m.handledByErrorsInjector(SetCompatibilityMode, w) {
			return
		}
		subjectName := strings.Split(req.URL.Path, "/")[2]
		compatibilityMode := readJsonBody(req, &schemareg.SetCompatibilityModeReq{})
		if validateCompatibilityMode(compatibilityMode.Compatibility) != nil {
			w.WriteHeader(422)
			_, _ = w.Write([]byte(`{"error_code":42203,"message":"Invalid compatibility level"}`))
		}
		if existingSubject, ok := m.Subjects[subjectName]; ok {
			existingSubject.CompatibilityMode = compatibilityMode.Compatibility
			w.WriteHeader(200)
			// TODO OK response
		} else {
			w.WriteHeader(500)
			// TODO response if subject doesn't exist?
		}
	}
}

func (m *SchemaRegMock) handledByErrorsInjector(api InjectOnApi, writer http.ResponseWriter) bool {
	maybeError := m.injectedErrors[api]
	if maybeError == nil {
		return false
	} else {
		writer.WriteHeader(maybeError.StatusCode)
		if len(maybeError.ResponseBody) > 0 {
			_, _ = writer.Write([]byte(maybeError.ResponseBody))
		}
		return true
	}
}

func (m *SchemaRegMock) Clear() {
	m.logger.Info("Removing previously registered subjects and schemas")
	m.Subjects = map[string]*Subject{}
	m.Schemas = map[int]string{}
	m.SoftDeletedSubjects = map[string]*Subject{}
	m.HardDeletedSubjects = map[string]*Subject{}
	m.injectedErrors = map[InjectOnApi]*InjectedError{}
}

type InjectOnApi string

const (
	RegisterSubject      InjectOnApi = "RegisterSubject"
	SetCompatibilityMode InjectOnApi = "SetCompatibilityMode"
	DeleteSubject        InjectOnApi = "DeleteSubject"
)

type InjectedError struct {
	OnApi        InjectOnApi
	StatusCode   int
	ResponseBody string
}

func (m *SchemaRegMock) InjectError(error InjectedError) {
	m.injectedErrors[error.OnApi] = &error
}

func (m *SchemaRegMock) DeleteSubject(subjectName string, permanent bool) ([]int, error) {
	if permanent {
		if existingSubject, ok := m.SoftDeletedSubjects[subjectName]; ok {
			m.HardDeletedSubjects[subjectName] = existingSubject
			return schemaVersions(existingSubject), nil
		} else {
			if m.Subjects[subjectName] != nil {
				return nil, fmt.Errorf(`{"error_code":40405,"message":"Subject '%s' was not deleted first before being permanently deleted"}`, subjectName)
			} else {
				return nil, fmt.Errorf(`{"error_code":40401,"message":"Subject '%s' not found."}`, subjectName)
			}
		}
	} else {
		if existingSubject, ok := m.Subjects[subjectName]; ok {
			delete(m.Subjects, subjectName)
			m.SoftDeletedSubjects[subjectName] = existingSubject
			return schemaVersions(existingSubject), nil
		} else {
			return nil, fmt.Errorf(`{"error_code":40401,"message":"Subject '%s' not found."}`, subjectName)
		}
	}
}

func schemaVersions(subject *Subject) []int {
	keys := make([]int, len(subject.SchemaRefs))

	i := 0
	for k := range subject.SchemaRefs {
		keys[i] = k
		i++
	}
	return keys
}

func validateCompatibilityMode(mode v1beta1.CompatibilityMode) error {
	// implement if needed ;)
	return nil
}
