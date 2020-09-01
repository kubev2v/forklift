package base

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	"io/ioutil"
	"net/http"
	liburl "net/url"
)

//
// Errors
var (
	ProviderNotSupported = liberr.New("provider not supported")
	ResourceNotSupported = liberr.New("resource not supported")
)

//
// Thin REST API client.
type Client struct {
	// Bearer token.
	Token string
	// Host <host>:<port>
	Host string
	// Parameters
	Params Params
}

//
// Http GET
func (c *Client) Get(path string, resource interface{}) (int, error) {
	header := http.Header{}
	if c.Token != "" {
		header["Authorization"] = []string{
			fmt.Sprintf("Bearer %s", c.Token),
		}
	}
	request := &http.Request{
		Method: http.MethodGet,
		Header: header,
		URL:    c.url(path),
	}
	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return -1, liberr.Wrap(err)
	}
	if response.StatusCode == http.StatusOK {
		defer response.Body.Close()
		content, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return -1, liberr.Wrap(err)
		}
		err = json.Unmarshal(content, resource)
		if err != nil {
			return -1, liberr.Wrap(err)
		}
		return response.StatusCode,
			nil
	}

	return response.StatusCode, nil
}

//
// Http POST
func (c *Client) Post(path string, resource interface{}) error {
	header := http.Header{}
	if c.Token != "" {
		header["Authorization"] = []string{
			fmt.Sprintf("Bearer %s", c.Token),
		}
	}
	body, _ := json.Marshal(resource)
	reader := bytes.NewReader(body)
	request := &http.Request{
		Body:   ioutil.NopCloser(reader),
		Method: http.MethodPost,
		Header: header,
		URL:    c.url(path),
	}
	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	if response.StatusCode == http.StatusCreated {
		defer response.Body.Close()
		content, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return err
		}
		err = json.Unmarshal(content, resource)
		if err != nil {
			return err
		}
		return nil
	}

	return errors.New(response.Status)
}

//
// Get the URL.
func (c *Client) url(path string) *liburl.URL {
	if c.Host == "" {
		c.Host = "localhost:8080"
	}
	path = (&Handler{}).Link(path, c.Params)
	url, _ := liburl.Parse(path)
	if url.Host == "" {
		url.Scheme = "http"
		url.Host = c.Host
	}

	return url
}
