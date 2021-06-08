package main

import (
	"encoding/json"
	"net/http"

	"github.com/yesoreyeram/grafana-infinity-datasource/pkg/infinity"
)

func (td *InfinityDatasource) proxyHandler(rw http.ResponseWriter, req *http.Request) {
	serverError := func(err error) {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
	client, err := getInstanceFromRequest(td.im, req)
	if err != nil {
		serverError(err)
		return
	}
	if req.Method == http.MethodPost {
		var query infinity.Query
		err = json.NewDecoder(req.Body).Decode(&query)
		if err != nil {
			serverError(err)
			return
		}
		if query.Source == "url" {
			response, err := client.c.GetResults(query)
			if err != nil {
				serverError(err)
				return
			}
			_, err = rw.Write(response)
			if err != nil {
				serverError(err)
			}
			return
		}
		if query.Source == "local-file" {
			response, err := client.c.GetLocalFileContent(query)
			if err != nil {
				http.Error(rw, err.Error(), http.StatusForbidden)
				return
			}
			// ignore error because ResponseWriter is error, we cannot write this error into it
			_, _ = rw.Write(response)
			return
		}
		http.Error(rw, "unknown query", http.StatusNotImplemented)
		return
	}
	http.Error(rw, "500 - Something bad happened! Invalid query.", http.StatusInternalServerError)
}
