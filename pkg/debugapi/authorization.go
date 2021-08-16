// Copyright 2020 The Sana Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package debugapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/ethsana/sana/pkg/jsonhttp"
)

func (s *Service) authorizationHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.authorization != `` && !strings.EqualFold(r.Header.Get(`Authorization`), s.authorization) {
			jsonhttp.InternalServerError(w, fmt.Errorf("authorization failed"))
			return
		}
		h.ServeHTTP(w, r)
	})
}
