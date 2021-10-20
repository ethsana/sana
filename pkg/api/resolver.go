// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/ethsana/sana"
	"github.com/ethsana/sana/pkg/jsonhttp"
	"github.com/gorilla/mux"
)

const lookupTxtPrefix = "sana://"

func (s *server) GatewayResolverHandler() http.Handler {
	router := mux.NewRouter()
	router.Handle("/sana/{address}/{path:.*}", http.HandlerFunc(s.bzzDownloadHandler))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := net.ParseIP(r.Host); ip != nil {
			jsonhttp.Forbidden(w, "")
			return
		}

		list, err := net.LookupTXT(r.Host)
		if err != nil || len(list) == 0 || !strings.HasPrefix(list[0], lookupTxtPrefix) {
			jsonhttp.Forbidden(w, "")
			return
		}
		address := strings.TrimPrefix(list[0], lookupTxtPrefix)
		r.URL.Path = fmt.Sprintf("/sana/%v%v", address, r.URL.Path)
		w.Header().Set("Server", fmt.Sprint("SANA/", sana.Version))
		router.ServeHTTP(w, r)
	})
}
