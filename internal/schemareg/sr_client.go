package schemareg

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"incubly.oss/kafka-schema-operator/api/v1beta1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/json"
)

//type BasicAuthCreds struct {
//	user string
//	pass string
//}

type SrClient struct {
	BaseUrl *url.URL
	//basicAuthCreds *BasicAuthCreds
	logger logr.Logger
}

type RegisterSchemaReq struct {
	Schema     string               `json:"schema"`
	SchemaType v1beta1.SchemaFormat `json:"schemaType,omitempty"`
}
type RegisterSchemaRes struct {
	Id int `json:"id"`
}

type SetCompatibilityModeReq struct {
	Compatibility v1beta1.CompatibilityMode `json:"compatibility"`
}

func (c *SrClient) RegisterSchema(subject string, req RegisterSchemaReq) (int, error) {
	jsonReq, _ := json.Marshal(req)
	jsonString, err := c.sendHttpRequest(
		"/subjects/"+subject+"/versions",
		"POST",
		string(jsonReq),
		map[string]string{})
	if err != nil {
		return 0, err
	} else {
		res := RegisterSchemaRes{}
		if err := json.Unmarshal([]byte(jsonString), &res); err != nil {
			return 0, err
		}
		c.logger.Info(fmt.Sprintf("Registered schema id=%d", res.Id))
		return res.Id, nil
	}
}

func (c *SrClient) DeleteSubject(subject string, permanent bool) error {
	_, err := c.sendHttpRequest(
		"/subjects/"+subject,
		"DELETE",
		"",
		map[string]string{
			"permanent": strconv.FormatBool(permanent),
		})
	return err
}

func (c *SrClient) SetCompatibilityMode(subject string, req SetCompatibilityModeReq) error {
	jsonReq, _ := json.Marshal(req)
	_, err := c.sendHttpRequest(
		"/config/"+subject,
		"PUT",
		string(jsonReq),
		map[string]string{})
	return err
}

func (c *SrClient) sendHttpRequest(
	uri string, httpMethod string, payload string, queryParams map[string]string) (string, error) {

	reqUrl := c.BaseUrl.JoinPath(uri)
	qParams := reqUrl.Query()
	for key, val := range queryParams {
		qParams.Set(key, val)
	}
	reqUrl.RawQuery = qParams.Encode()

	req, err := http.NewRequest(
		httpMethod,
		reqUrl.String(),
		strings.NewReader(payload))

	if err != nil {
		c.logger.Error(err, "Failed to create request to schema-registry")
		return "", err
	}
	req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")

	//if c.basicAuthCreds != nil {
	//	req.SetBasicAuth(c.basicAuthCreds.user, c.basicAuthCreds.pass)
	//}

	c.logger.Info(fmt.Sprintf(
		"> HTTP request: %s %s, payload: %s", req.Method, req.URL, payload))
	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		c.logger.Error(err, "Failed to send request to schema-registry")
		return "", err
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			c.logger.Error(err, "Failed to close response body reader")
		}
	}(resp.Body)

	c.logger.Info(fmt.Sprintf("< HTTP response: %s", resp.Status))

	if resp.StatusCode < 300 {
		return c.handleHttpSuccess(resp)
	} else {
		return "", c.handleHttpError(resp)
	}
}

func (c *SrClient) handleHttpSuccess(resp *http.Response) (string, error) {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if bodyBytes == nil {
		return "", nil
	} else {
		return string(bodyBytes), nil
	}
}

func (c *SrClient) handleHttpError(resp *http.Response) error {
	if resp.StatusCode < 300 {
		return nil
	} else {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.logger.Error(err, "Cannot read http response body")
		}
		if bodyBytes == nil {
			return fmt.Errorf("schema registry error: %s", resp.Status)
		} else {
			return fmt.Errorf("schema registry error: %s, %s", resp.Status, string(bodyBytes))
		}
	}
}

func NewClient(schemaReg *v1beta1.SchemaRegistry, logger logr.Logger) (*SrClient, error) {
	baseUrl, err := resolveBaseUrl(schemaReg)
	if err != nil {
		logger.Error(err, "Failed to resolve base url for schema registry")
		return nil, err
	}

	client := &SrClient{
		BaseUrl: baseUrl,
		logger:  logger,
	}

	return client, nil
}

func resolveBaseUrl(schemaReg *v1beta1.SchemaRegistry) (*url.URL, error) {
	if schemaReg != nil && len(schemaReg.BaseUrl) > 0 {
		return url.Parse(schemaReg.BaseUrl)
	} else {
		defaultBaseUrl := os.Getenv("SCHEMA_REGISTRY_BASE_URL")
		if len(defaultBaseUrl) > 0 {
			return url.Parse(defaultBaseUrl)
		} else {
			return nil, fmt.Errorf("base URL for schema registry not set")
		}
	}
}
