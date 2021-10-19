package api

import "net/http"

func (s *server) sanaUploadHandler(w http.ResponseWriter, r *http.Request) {
	s.bzzUploadHandler(w, r)
}

func (s *server) sanaDownloadHandler(w http.ResponseWriter, r *http.Request) {
	s.bzzDownloadHandler(w, r)
}

func (s *server) sanaPatchHandler(w http.ResponseWriter, r *http.Request) {
	s.bzzPatchHandler(w, r)
}
