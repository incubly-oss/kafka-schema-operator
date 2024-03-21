package schemareg

import (
	er "errors"
	"github.com/go-logr/logr"
	"io"
	"k8s.io/apimachinery/pkg/util/json"
	kafkaschemaoperatorv2beta1 "kafka-schema-operator/api/v2beta1"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type BasicAuthCreds struct {
	user string
	pass string
}

type SrClient struct {
	baseUrl        string
	basicAuthCreds *BasicAuthCreds
	logger         logr.Logger
}

type RegisterSchemaReq struct {
	Schema     string `json:"schema"`
	SchemaType string `json:"schemaType,omitempty"`
}

type SetCompatibilityModeReq struct {
	Compatibility string `json:"compatibility"`
}

func (c *SrClient) RegisterSchema(subject string, req RegisterSchemaReq) error {
	jsonReq, _ := json.Marshal(req)
	return c.sendHttpRequest(
		"/subjects/"+subject+"/versions",
		"POST",
		string(jsonReq))
}

func (c *SrClient) DeleteSubject(subject string) error {
	return c.sendHttpRequest(
		"/subjects/"+subject,
		"DELETE",
		"")
}

func (c *SrClient) SetCompatibilityMode(subject string, req SetCompatibilityModeReq) error {
	jsonReq, _ := json.Marshal(req)
	return c.sendHttpRequest(
		"/config/"+subject,
		"PUT",
		string(jsonReq))
}

func (c *SrClient) sendHttpRequest(url string, httpMethod string, payload string) error {
	httpReq, err := http.NewRequest(
		httpMethod,
		c.baseUrl+url,
		strings.NewReader(payload))

	if err != nil {
		c.logger.Error(err, "Failed to create request to schema-registry")
		return err
	}
	httpReq.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")

	if c.basicAuthCreds != nil {
		httpReq.SetBasicAuth(c.basicAuthCreds.user, c.basicAuthCreds.pass)
	}

	httpResp, err := http.DefaultClient.Do(httpReq)

	if err != nil {
		c.logger.Error(err, "Failed to send request to schema-registry")
		return err
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			c.logger.Error(err, "Failed to close response body reader")
		}
	}(httpResp.Body)

	if httpResp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(httpResp.Body)
		if err != nil {
			c.logger.Error(err, "Cannot read http response body")
		}
		c.logger.Info("Schema registry returned error")
		c.logger.Info("Statuscode: " + strconv.Itoa(httpResp.StatusCode))
		c.logger.Info("Body response: " + string(bodyBytes))
		return er.New("Schema registry error: " + string(bodyBytes))
	}

	return nil
}

func NewClient(schemaReg *kafkaschemaoperatorv2beta1.SchemaRegistry, maybePassword string, logger logr.Logger) (*SrClient, error) {
	baseUrl, err := resolveBaseUrl(schemaReg)
	if err != nil {
		logger.Error(err, "Failed to resolve base url for schema registry")
		return nil, err
	}

	basicAuthCreds := resolveBasicAuthCreds(schemaReg, maybePassword)

	client := &SrClient{
		baseUrl:        baseUrl,
		basicAuthCreds: basicAuthCreds,
		logger:         logger,
	}

	return client, nil
}

func resolveBaseUrl(schemaReg *kafkaschemaoperatorv2beta1.SchemaRegistry) (string, error) {
	var srHost string
	if schemaReg != nil && len(schemaReg.Host) > 0 {
		srHost = schemaReg.Host
	} else {
		srHost = os.Getenv("SCHEMA_REGISTRY_HOST")
	}

	var srPort string
	if schemaReg != nil && schemaReg.Port > 0 {
		srPort = strconv.Itoa(schemaReg.Port)
	} else {
		srPort = os.Getenv("SCHEMA_REGISTRY_PORT")
	}
	if len(srHost) == 0 || len(srPort) == 0 {
		return "", er.New("schema registry host or port is not set")
	}

	baseUrl := "http://" + srHost + ":" + srPort
	return baseUrl, nil
}

func resolveBasicAuthCreds(reg *kafkaschemaoperatorv2beta1.SchemaRegistry, password string) *BasicAuthCreds {
	if reg != nil && len(reg.User) > 0 && len(password) > 0 {
		return &BasicAuthCreds{
			user: reg.User,
			pass: password,
		}
	} else if len(os.Getenv("SCHEMA_REGISTRY_KEY")) > 0 && len(os.Getenv("SCHEMA_REGISTRY_SECRET")) > 0 {
		return &BasicAuthCreds{
			user: os.Getenv("SCHEMA_REGISTRY_KEY"),
			pass: os.Getenv("SCHEMA_REGISTRY_SECRET"),
		}
	} else {
		return nil
	}
}
