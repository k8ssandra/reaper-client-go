package reaper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/go-querystring/query"
)

func (c *client) doGet(
	ctx context.Context,
	path string,
	queryParams interface{},
	expectedStatuses ...int,
) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodGet, path, queryParams, nil, expectedStatuses...)
}

func (c *client) doPost(
	ctx context.Context,
	path string,
	queryParams interface{},
	formData interface{},
	expectedStatuses ...int,
) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodPost, path, queryParams, formData, expectedStatuses...)
}

func (c *client) doPut(
	ctx context.Context,
	path string,
	queryParams interface{},
	formData interface{},
	expectedStatuses ...int,
) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodPut, path, queryParams, formData, expectedStatuses...)
}

func (c *client) doDelete(
	ctx context.Context,
	path string,
	queryParams interface{},
	expectedStatuses ...int,
) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodDelete, path, queryParams, nil, expectedStatuses...)
}

func (c *client) doHead(
	ctx context.Context,
	path string,
	queryParams interface{},
	expectedStatuses ...int,
) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodHead, path, queryParams, nil, expectedStatuses...)
}

func (c *client) doRequest(
	ctx context.Context,
	method string,
	path string,
	queryParams interface{},
	formData interface{},
	expectedStatuses ...int,
) (*http.Response, error) {
	u := c.resolveURL(path)
	if queryParams != nil {
		queryValues, err := c.paramSourceToValues(queryParams)
		if err != nil {
			return nil, err
		}
		u.RawQuery = queryValues.Encode()
	}
	var body string
	var bodyReader io.Reader
	if formData != nil {
		formValues, err := c.paramSourceToValues(formData)
		if err != nil {
			return nil, err
		}
		body = formValues.Encode()
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, err
	}
	if formData != nil {
		c.addFormHeaders(req, body)
	}
	// TODO authentication headers
	c.addCommonHeaders(req)
	res, err := c.httpClient.Do(req)
	if err == nil {
		err = c.checkResponseStatus(res, expectedStatuses...)
	}
	return res, err
}

func (c *client) mergeParamSources(paramSources ...interface{}) (*url.Values, error) {
	mergedValues := url.Values{}
	for _, paramSource := range paramSources {
		paramSourceValues, err := c.paramSourceToValues(paramSource)
		if err != nil {
			return nil, err
		}
		for key, values := range *paramSourceValues {
			mergedValues[key] = append(mergedValues[key], values...)
		}
	}
	return &mergedValues, nil
}

func (c *client) paramSourceToValues(paramSource interface{}) (*url.Values, error) {
	if paramSource == nil {
		return nil, nil
	}
	if values, ok := paramSource.(*url.Values); ok {
		return values, nil
	}
	if m, ok := paramSource.(map[string]string); ok {
		values := make(url.Values)
		for key, val := range m {
			values.Add(key, val)
		}
		return &values, nil
	}
	if m, ok := paramSource.(map[string][]string); ok {
		values := url.Values(m)
		return &values, nil
	}
	values, err := query.Values(paramSource)
	if err != nil {
		return nil, err
	}
	return &values, nil
}

func (c *client) resolveURL(path string) *url.URL {
	rel := &url.URL{Path: path}
	u := c.baseURL.ResolveReference(rel)
	return u
}

func (c *client) addFormHeaders(req *http.Request, requestBody string) {
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(requestBody)))
}

func (c *client) addCommonHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json;q=0.9,text/plain")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	if c.jSessionId != nil {
		req.Header.Set("Cookie", fmt.Sprintf("JSESSIONID=%s", *c.jSessionId))
	}

	if c.jwt != nil {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *c.jwt))
	}
}

func (c *client) readBodyAsString(res *http.Response) (string, error) {
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *client) readBodyAsJson(res *http.Response, v interface{}) error {
	err := json.NewDecoder(res.Body).Decode(v)
	_ = res.Body.Close()
	return err
}

func (c *client) checkResponseStatus(res *http.Response, expectedStatuses ...int) error {
	if len(expectedStatuses) == 0 {
		// the caller didn't specify any status: assume they will deal with statuses themselves
		return nil
	}
	for _, status := range expectedStatuses {
		if res.StatusCode == status {
			return nil
		}
	}
	return c.bodyToError(res)
}

func (c *client) bodyToError(res *http.Response) error {
	message, err := c.readBodyAsString(res)
	if message != "" && err == nil {
		contentType := res.Header.Get("Content-Type")
		if contentType == "application/json" {
			payload := &errorPayload{}
			err = json.NewDecoder(strings.NewReader(message)).Decode(payload)
			if err == nil && payload.Message != "" {
				message = payload.Message
			}
		}
	} else {
		message = http.StatusText(res.StatusCode)
	}
	return fmt.Errorf("%s (HTTP status %d)", message, res.StatusCode)
}

type errorPayload struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
