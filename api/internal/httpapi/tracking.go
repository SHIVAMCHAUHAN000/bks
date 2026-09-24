package httpapi

import (
	"net/http"
	"strconv"
	"strings"
)

var transparentGIF = []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;")

func (s *Server) trackingHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/track/open/"), 10, 64)
	if err == nil {
		s.leads.MarkOpened(id)
	}
	w.Header().Set("Content-Type", "image/gif")
	w.Write(transparentGIF)
}
