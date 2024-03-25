package schemareg_mock

import (
	"fmt"
	"github.com/riferrei/srclient"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

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
	CompatibilityMode srclient.CompatibilityLevel
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
		m.createSubject(),
	)
	server.RouteToHandler(
		"DELETE",
		regexp.MustCompile(`^/subjects/[a-zA-Z0-9-_.]+$`),
		m.deleteSubject(),
	)
	server.RouteToHandler(
		"PUT",
		regexp.MustCompile(`^/config/[a-zA-Z0-9-_.]+$`),
		m.changeCompatibilityLevel(),
	)
	server.RouteToHandler(
		"GET",
		regexp.MustCompile(`^/schemas/ids/[0-9]+$`),
		m.getSchemaById(),
	)

	return server
}

type createSchemaReq struct {
	Schema     string              `json:"schema"`
	SchemaType srclient.SchemaType `json:"schemaType,omitempty"`
}
type getSchemaRes struct {
	Schema string `json:"schema"`
}

type changeCompatibilityLevelReq struct {
	Compatibility srclient.CompatibilityLevel `json:"compatibility"`
}

func (m *SchemaRegMock) createSubject() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if m.handledByErrorsInjector(CreateSubject, w) {
			return
		}

		subjectName := strings.Split(req.URL.Path, "/")[2]

		m.logger.Info("Schema-reg: creating subject " + subjectName)

		createSchemaReq := readJsonBody(req, &createSchemaReq{})

		schema, parseSchemaErr := parseSchema(*createSchemaReq)
		if parseSchemaErr != nil {
			w.WriteHeader(422)
			_, _ = w.Write([]byte(`{"error_code":42201,"message":"Invalid schema"}`))
			return
		}

		if _, ok := m.Subjects[subjectName]; !ok {
			m.Subjects[subjectName] = &Subject{
				CompatibilityMode: srclient.Backward,
				SchemaRefs:        []SchemaRef{},
			}
		}

		schemaId := m.registerSchema(schema)
		m.Subjects[subjectName].setSchemaAsCurrentVersion(schemaId)

		_, _ = w.Write([]byte(fmt.Sprintf(`{"id": %d}`, schemaId)))
		w.WriteHeader(200)
	}
}

func (m *SchemaRegMock) getSchemaById() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		schemaIdString := strings.Split(req.URL.Path, "/")[3]
		schemaId, err := strconv.Atoi(schemaIdString)
		if err != nil {
			m.logger.Error(err, fmt.Sprintf("Failed to parse schemaId=%s to int", schemaIdString))
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error_code":404,"message":"HTTP 404 Not Found"}`))
		}
		maybeSchema := m.Schemas[schemaId]
		if len(maybeSchema) == 0 {
			w.WriteHeader(404)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"error_code":40403,"message":"Schema %d not found"}`, schemaId)))
		} else {
			res := getSchemaRes{Schema: maybeSchema}
			bytes, _ := json.Marshal(res)
			w.WriteHeader(200)
			_, _ = w.Write(bytes)
		}
	}
}

func parseSchema(req createSchemaReq) (string, error) {
	if req.SchemaType == srclient.Avro || len(req.SchemaType) == 0 {
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

func (m *SchemaRegMock) deleteSubject() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if m.handledByErrorsInjector(DeleteSubject, w) {
			return
		}
		subjectName := strings.Split(req.URL.Path, "/")[2]
		queryParams, _ := url.ParseQuery(req.URL.RawQuery)
		isHardDelete := queryParams.Get("permanent") == "true"
		if isHardDelete {
			if existingSubject, ok := m.SoftDeletedSubjects[subjectName]; ok {
				m.HardDeletedSubjects[subjectName] = existingSubject
				w.WriteHeader(200)
				// TODO OK response
			} else {
				w.WriteHeader(500)
				// TODO response if subject doesn't exist?
			}
		} else {
			if existingSubject, ok := m.Subjects[subjectName]; ok {
				delete(m.Subjects, subjectName)
				m.SoftDeletedSubjects[subjectName] = existingSubject
				w.WriteHeader(200)
				// TODO OK response
			} else {
				w.WriteHeader(500)
				// TODO response if subject doesn't exist?
			}
		}
	}
}

func (m *SchemaRegMock) changeCompatibilityLevel() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if m.handledByErrorsInjector(ChangeCompatibilityLevel, w) {
			return
		}
		subjectName := strings.Split(req.URL.Path, "/")[2]
		compatibilityMode := readJsonBody(req, &changeCompatibilityLevelReq{})
		if validateCompatibilityMode(compatibilityMode.Compatibility) != nil {
			w.WriteHeader(422)
			_, _ = w.Write([]byte(`{"error_code":42203,"message":"Invalid compatibility level"}`))
		}
		if existingSubject, ok := m.Subjects[subjectName]; ok {
			existingSubject.CompatibilityMode = compatibilityMode.Compatibility
			w.WriteHeader(200)
			res, _ := json.Marshal(compatibilityMode)
			_, _ = w.Write(res)
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
	m.logger.Info("Removing previously created subjects and schemas")
	m.Subjects = map[string]*Subject{}
	m.Schemas = map[int]string{}
	m.SoftDeletedSubjects = map[string]*Subject{}
	m.HardDeletedSubjects = map[string]*Subject{}
	m.injectedErrors = map[InjectOnApi]*InjectedError{}
}

type InjectOnApi string

const (
	CreateSubject            InjectOnApi = "CreateSubject"
	ChangeCompatibilityLevel InjectOnApi = "ChangeCompatibilityLevel"
	DeleteSubject            InjectOnApi = "DeleteSubject"
)

type InjectedError struct {
	OnApi        InjectOnApi
	StatusCode   int
	ResponseBody string
}

func (m *SchemaRegMock) InjectError(error InjectedError) {
	m.injectedErrors[error.OnApi] = &error
}

func validateCompatibilityMode(mode srclient.CompatibilityLevel) error {
	// implement if needed ;)
	return nil
}
