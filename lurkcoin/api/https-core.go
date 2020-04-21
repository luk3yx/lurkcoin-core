//
// lurkcoin
// Copyright © 2020 by luk3yx
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//

package api

import (
	"encoding/json"
	"errors"
	"github.com/julienschmidt/httprouter"
	"io"
	"lurkcoin"
	"net/http"
	"strings"
)

var c0 = lurkcoin.CurrencyFromInt64(0)

// A HTTP server wrapper
type HTTPRequest struct {
	Server        *lurkcoin.Server
	Database      lurkcoin.Database
	DbTransaction *lurkcoin.DatabaseTransaction
	Request       *http.Request
	Params        httprouter.Params
}

func MakeHTTPRequest(db lurkcoin.Database, request *http.Request, params httprouter.Params) *HTTPRequest {
	return &HTTPRequest{nil, db, nil, request, params}
}

type HTTPHandler func(*HTTPRequest) (interface{}, error)

// Unmarshals JSON sent in the HTTP request into v.
func (self *HTTPRequest) Unmarshal(v interface{}) error {
	// Ensure the Content-Type header is correct.
	contentType := self.Request.Header.Get("Content-Type")
	if i := strings.IndexByte(contentType, ';'); i >= 0 {
		contentType = contentType[:i]
	}
	if contentType != "" && contentType != "application/json" &&
		!(strings.HasPrefix(contentType, "application/") &&
			strings.HasSuffix(contentType, "+json")) {
		return errors.New("ERR_INVALIDREQUEST")
	}

	length := self.Request.ContentLength

	// Default to the maximum length plus one.
	if length < 0 {
		length = 4097
	} else if length > 4096 {
		return errors.New("ERR_PAYLOADTOOLARGE")
	}

	raw := make([]byte, length)
	actual_length, _ := self.Request.Body.Read(raw)

	if actual_length < 3 {
		return errors.New("ERR_INVALIDREQUEST")
	} else if actual_length > 4096 {
		return errors.New("ERR_PAYLOADTOOLARGE")
	}

	json_err := json.Unmarshal(raw[:actual_length], v)
	if json_err != nil {
		return errors.New("ERR_INVALIDREQUEST")
	}
	return nil
}

func (self *HTTPRequest) AbortTransaction() {
	if self.DbTransaction != nil {
		self.DbTransaction.Abort()
		self.DbTransaction = nil
	}
}

func (self *HTTPRequest) FinishTransaction() {
	if self.DbTransaction != nil {
		self.DbTransaction.Finish()
		self.DbTransaction = nil
	}
}

func authenticateRequest(r *http.Request, db lurkcoin.Database, otherServers ...string) (bool, *lurkcoin.DatabaseTransaction, *lurkcoin.Server) {
	// Get the username and token
	username, token, ok := r.BasicAuth()
	if !ok {
		return false, nil, nil
	}

	return lurkcoin.AuthenticateRequest(db, username, token, otherServers)
}

func (self *HTTPRequest) Authenticate(otherServers ...string) error {
	// Get the username and token
	username, token, ok := self.Request.BasicAuth()
	if !ok {
		return errors.New("ERR_INVALIDREQUEST")
	}

	authed, tr, server := lurkcoin.AuthenticateRequest(
		self.Database,
		username,
		token,
		otherServers,
	)

	if !authed {
		return errors.New("ERR_INVALIDLOGIN")
	}

	self.Server = server
	self.DbTransaction = tr
	return nil
}

func securityTxt(w http.ResponseWriter, r *http.Request,
	_ httprouter.Params) {
	io.WriteString(w, "# lurkcoin version: "+lurkcoin.VERSION+"\n")
	io.WriteString(w, "# Source: "+lurkcoin.SOURCE_URL+"\n")
	io.WriteString(w, "Contact: "+lurkcoin.REPORT_SECURITY+"\n")
}

func makeRedirect(router *httprouter.Router, source, target string) {
	f := func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		http.Redirect(w, r, target, http.StatusFound)
	}
	router.GET(source, f)
}

func MakeHTTPRouter(db lurkcoin.Database, config *Config) *httprouter.Router {
	router := httprouter.New()
	router.GET("/.well-known/security.txt", securityTxt)

	// Add custom redirects
	for source, target := range config.Redirects {
		makeRedirect(router, source, target)
	}

	// Don't give up (or let down) bots
	if _, exists := config.Redirects["/wp-login.php"]; !exists {
		makeRedirect(router, "/wp-login.php",
			"https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	}

	if config.AdminPages.Enable && config.AdminPages.Users != nil {
		addAdminPages(router, db, config.AdminPages.Users)
	}
	if config.MinAPIVersion > 3 {
		return router
	}
	addV3API(router, db)
	if config.MinAPIVersion > 2 {
		return router
	}
	addV2API(router, db, config.Name)
	return router
}
