package infinity

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type Client struct {
	Settings   InfinitySettings
	HttpClient *http.Client
}

func GetTLSConfigFromSettings(settings InfinitySettings) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: settings.InsecureSkipVerify,
		ServerName:         settings.ServerName,
	}
	if settings.TLSClientAuth {
		if settings.TLSClientCert == "" || settings.TLSClientKey == "" {
			return nil, errors.New("invalid Client cert or key")
		}
		cert, err := tls.X509KeyPair([]byte(settings.TLSClientCert), []byte(settings.TLSClientKey))
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	if settings.TLSAuthWithCACert && settings.TLSCACert != "" {
		caPool := x509.NewCertPool()
		ok := caPool.AppendCertsFromPEM([]byte(settings.TLSCACert))
		if !ok {
			return nil, errors.New("invalid TLS CA certificate")
		}
		tlsConfig.RootCAs = caPool
	}
	return tlsConfig, nil
}

func NewClient(settings InfinitySettings) (client *Client, err error) {
	tlsConfig, err := GetTLSConfigFromSettings(settings)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	httpClient := &http.Client{
		Transport: transport,
	}
	return &Client{
		Settings:   settings,
		HttpClient: httpClient,
	}, err
}

func replaceSecret(input string, settings InfinitySettings) string {
	for key, value := range settings.SecureQueryFields {
		input = strings.ReplaceAll(input, fmt.Sprintf("${__qs.%s}", key), value)
	}
	return input
}

func GetQueryURL(settings InfinitySettings, query Query) string {
	urlString := fmt.Sprintf("%s%s", settings.URL, query.URL)
	urlString = replaceSecret(urlString, settings)
	u, err := url.Parse(urlString)
	if err != nil {
		return urlString
	}
	q := u.Query()
	for _, param := range query.URLOptions.Params {
		value := replaceSecret(param.Value, settings)
		q.Set(param.Key, value)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func getRequest(settings InfinitySettings, body io.Reader, query Query) (req *http.Request, err error) {
	url := GetQueryURL(settings, query)
	switch query.URLOptions.Method {
	case "POST":
		req, err = http.NewRequest("POST", url, body)
	default:
		req, err = http.NewRequest("GET", url, nil)
	}
	if settings.BasicAuthEnabled && (settings.UserName != "" || settings.Password != "") {
		req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(settings.UserName+":"+settings.Password)))
	}
	req.Header.Set("User-Agent", "Grafana")
	if query.Type == "json" || query.Type == "graphql" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, header := range query.URLOptions.Headers {
		value := replaceSecret(header.Value, settings)
		req.Header.Set(header.Key, value)
	}
	for key, value := range settings.CustomHeaders {
		req.Header.Set(key, value)
	}
	return req, err
}

func (client *Client) req(url string, body *strings.Reader, settings InfinitySettings, query Query) ([]byte, error) {
	req, _ := getRequest(settings, body, query)
	res, err := client.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error getting response from %s", url)
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return nil, errors.New(res.Status)
	}
	bodyBytes, err := io.ReadAll(res.Body)
	return bodyBytes, err
}

func (client *Client) GetResults(query Query) ([]byte, error) {
	method := query.URLOptions.Method
	switch method {
	case http.MethodGet:
		return client.req(query.URL, nil, client.Settings, query)
	case http.MethodPost:
		body := strings.NewReader(query.URLOptions.Data)
		if query.Type == "graphql" {
			jsonData := map[string]string{
				"query": query.URLOptions.Data,
			}
			jsonValue, _ := json.Marshal(jsonData)
			body = strings.NewReader(string(jsonValue))
		}
		return client.req(query.URL, body, client.Settings, query)
	default:
		return nil, errors.New(fmt.Sprintf("unsupported method %s", method))
	}
}

func (client *Client) GetLocalFileContent(query Query) ([]byte, error) {
	filePath := strings.TrimSpace(query.URL)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return content, nil
}
