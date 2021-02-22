//
// lurkcoin HTTPS API (version 3)
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

// API documentation:
// https://gist.github.com/luk3yx/7a07f8b307c9afbcf94cf47d7f41d9cb

package api

import (
	"encoding/json"
	"errors"
	"github.com/julienschmidt/httprouter"
	"github.com/luk3yx/lurkcoin-core/lurkcoin"
	"net/http"
	"strings"
)

func v3WrapHTTPHandler(db lurkcoin.Database, autoLogin bool,
	handlerFunc HTTPHandler) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		req := MakeHTTPRequest(db, r, params)
		defer req.AbortTransaction()

		var result interface{}
		var err error
		if !autoLogin || req.Authenticate() == nil {
			result, err = handlerFunc(req)
		} else {
			err = errors.New("ERR_INVALIDLOGIN")
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		res := make(map[string]interface{})
		if err == nil {
			req.FinishTransaction()
			res["success"] = true
			res["result"] = result
			w.WriteHeader(http.StatusOK)
		} else {
			req.AbortTransaction()
			var c int
			res["success"] = false
			res["error"], res["message"], c = lurkcoin.LookupError(err.Error())

			// Workaround for limitations of Minetest's HTTP API
			if isYes(r.Header.Get("X-Force-OK")) {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(c)
			}
		}

		// TODO: Possibly write JSON directly to the ResponseWriter.
		raw, enc_err := json.Marshal(res)
		if enc_err != nil {
			raw = []byte(`{"success":false,"error":"ERR_INTERNALERROR","message":"Internal error!"}`)
		}
		w.Write(raw)
	}
}

func v3Get(router *httprouter.Router, db lurkcoin.Database, url string,
	requireLogin bool, f HTTPHandler) {
	f2 := v3WrapHTTPHandler(db, requireLogin, f)
	url = "/v3/" + url
	router.GET(url, f2)
	router.POST(url, f2)
}

func v3Post(router *httprouter.Router, db lurkcoin.Database, url string,
	requireLogin bool, f HTTPHandler) {
	router.POST("/v3/"+url, v3WrapHTTPHandler(db, requireLogin, f))
}

func v3Put(router *httprouter.Router, db lurkcoin.Database, url string,
	requireLogin bool, f HTTPHandler) {
	f2 := v3WrapHTTPHandler(db, requireLogin, f)
	router.PUT("/v3/"+url, f2)
	router.POST("/v3/set_"+url, f2)
}

func addV3API(router *httprouter.Router, db lurkcoin.Database) {
	v3Get(router, db, "summary", true,
		func(r *HTTPRequest) (interface{}, error) {
			return r.Server.GetSummary(), nil
		})

	v3Post(router, db, "pay", false,
		func(r *HTTPRequest) (transaction interface{}, err error) {
			var p struct {
				Source        string            `json:"source"`
				Target        string            `json:"target"`
				TargetServer  string            `json:"target_server"`
				Amount        lurkcoin.Currency `json:"amount"`
				LocalCurrency bool              `json:"local_currency"`
			}
			err = r.Unmarshal(&p)
			if err != nil {
				return
			}
			err = r.Authenticate(p.TargetServer)
			if err != nil {
				return
			}
			if p.Amount.IsNil() {
				err = errors.New("ERR_INVALIDAMOUNT")
				return
			}
			targetServer, ok := r.DbTransaction.GetCachedServer(p.TargetServer)
			if !ok {
				err = errors.New("ERR_SERVERNOTFOUND")
				return
			}
			transaction, err = r.Server.Pay(p.Source, p.Target, targetServer,
				p.Amount, p.LocalCurrency, true)
			return
		})

	v3Get(router, db, "balance", true,
		func(r *HTTPRequest) (interface{}, error) {
			return r.Server.GetBalance(), nil
		})

	v3Get(router, db, "history", true,
		func(r *HTTPRequest) (interface{}, error) {
			return r.Server.GetHistory(), nil
		})

	v3Post(router, db, "exchange_rates", false,
		func(r *HTTPRequest) (interface{}, error) {
			var p struct {
				Source string `json:"source"`
				Target string `json:"target"`
				Amount lurkcoin.Currency
			}
			r.Unmarshal(&p)
			if p.Amount.IsNil() {
				return nil, errors.New("ERR_INVALIDAMOUNT")
			}
			return lurkcoin.GetExchangeRate(r.Database, p.Source, p.Target,
				p.Amount)
		})

	v3Get(router, db, "pending_transactions", true,
		func(r *HTTPRequest) (interface{}, error) {
			return r.Server.GetPendingTransactions(), nil
		})

	type transactionList struct {
		TransactionIDs []string `json:"transactions"`
	}
	v3Post(router, db, "acknowledge_transactions", true,
		func(r *HTTPRequest) (interface{}, error) {
			var p transactionList
			r.Unmarshal(&p)
			for _, id := range p.TransactionIDs {
				r.Server.RemovePendingTransaction(id)
			}
			return nil, nil
		})

	v3Post(router, db, "reject_transactions", true,
		func(r *HTTPRequest) (interface{}, error) {
			var p transactionList
			r.Unmarshal(&p)
			for _, id := range p.TransactionIDs {
				r.Server.RejectPendingTransaction(id, r.DbTransaction)
			}
			return nil, nil
		})

	v3Get(router, db, "target_balance", true,
		func(r *HTTPRequest) (interface{}, error) {
			return r.Server.GetTargetBalance(), nil
		})

	v3Put(router, db, "target_balance", true,
		func(r *HTTPRequest) (interface{}, error) {
			var p struct {
				TargetBalance lurkcoin.Currency `json:"target_balance"`
			}
			err := r.Unmarshal(&p)
			if err != nil {
				return nil, errors.New("ERR_INVALIDREQUEST")
			}
			if p.TargetBalance.IsNil() {
				return nil, errors.New("ERR_INVALIDAMOUNT")
			}
			ok := r.Server.SetTargetBalance(p.TargetBalance)
			if !ok {
				return nil, errors.New("ERR_INVALIDAMOUNT")
			}
			return nil, nil
		})

	v3Get(router, db, "webhook_url", true,
		func(r *HTTPRequest) (interface{}, error) {
			if r.Server.WebhookURL == "" {
				return nil, nil
			}
			return r.Server.WebhookURL, nil
		})

	v3Get(router, db, "version", false,
		func(r *HTTPRequest) (interface{}, error) {
			return map[string]interface{}{
				"version":   lurkcoin.VERSION,
				"copyright": strings.Split(lurkcoin.COPYRIGHT, "\n"),
				"license":   "AGPLv3",
				"source":    lurkcoin.SOURCE_URL,
			}, nil
		})
}
