//
// lurkcoin HTTPS API (version 2)
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
// https://gist.github.com/luk3yx/8028cedb3bfb282d9ba3f2d1c7871231

// +build !lurkcoin.disablev2api

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"lurkcoin"
	"math/big"
	"net/http"
	"strconv"
	"strings"
)

type v2Form interface {
	Get(string) string
}

type v2MapForm struct {
	form map[string]json.Number
}

func (self *v2MapForm) Get(key string) string {
	res, ok := self.form[key]
	if ok {
		return string(res)
	} else {
		return ""
	}
}

var c1 = lurkcoin.CurrencyFromInt64(1)
var f0 = big.NewFloat(0)
var f500k = big.NewFloat(500000)

func v2GetQuery(r *http.Request) v2Form {
	err := r.ParseForm()
	if err == nil && len(r.Form) > 0 {
		return r.Form
	}

	// Because json.Number extends string it can be used for strings.
	form := make(map[string]json.Number)
	(&HTTPRequest{Request: r}).Unmarshal(&form)
	return &v2MapForm{form}
}

func (self *HTTPRequest) AuthenticateV2(query v2Form, otherServers ...string) error {
	// Get the username and token
	username := query.Get("name")
	token := query.Get("token")

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

type v2HTTPHandler func(*HTTPRequest, v2Form) (interface{}, error)

func v2WrapHTTPHandler(db lurkcoin.Database, autoLogin bool,
	handlerFunc v2HTTPHandler) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request,
		params httprouter.Params) {
		req := MakeHTTPRequest(db, r, params)
		defer req.AbortTransaction()
		query := v2GetQuery(r)

		var result interface{}
		var err error
		if !autoLogin || req.AuthenticateV2(query) == nil {
			result, err = handlerFunc(req, query)
		} else {
			err = errors.New("ERR_INVALIDLOGIN")
		}

		var res []byte
		if err == nil {
			req.FinishTransaction()
			if s, ok := result.(string); ok {
				res = []byte(s)
			} else {
				var enc_err error
				res, enc_err = json.Marshal(result)
				if enc_err == nil {
					w.Header().Set("Content-Type",
						"application/json; charset=utf-8")
				} else {
					res = []byte("ERROR: Internal error!")
				}
			}
			w.WriteHeader(http.StatusOK)
		} else {
			req.AbortTransaction()
			var c int
			var msg string
			_, msg, c = lurkcoin.LookupError(err.Error())
			res = []byte("ERROR: " + msg)
			if c != 401 && query.Get("force_200") == "200" {
				c = 200
			}
			w.WriteHeader(c)
		}

		w.Write(res)
	}
}

func v2Post(router *httprouter.Router, db lurkcoin.Database, url string,
	autoLogin bool, f v2HTTPHandler) {
	url = "/v2/" + url
	f2 := v2WrapHTTPHandler(db, autoLogin, f)
	router.GET(url, f2)
	router.POST(url, f2)
}

func v2IsYes(s string) bool {
	switch strings.ToLower(s) {
	case "true", "yes", "y", "1":
		return true
	default:
		return false
	}
}

func addV2API(router *httprouter.Router, db lurkcoin.Database,
	lurkcoinName string) {

	v2Post(router, db, "summary", true,
		func(r *HTTPRequest, _ v2Form) (interface{}, error) {
			summary := r.Server.GetSummary()
			return map[string]interface{}{
				"uid":           summary.UID,
				"bal":           summary.Bal,
				"balance":       summary.Balance,
				"history":       lurkcoin.GetV2History(summary, false),
				"server":        true,
				"interest_rate": summary.InterestRate,
			}, nil
		})

	v2Post(router, db, "pay", false,
		func(r *HTTPRequest, f v2Form) (interface{}, error) {
			amount, err := lurkcoin.ParseCurrency(f.Get("amount"))
			if err != nil {
				return nil, err
			}

			targetServerName := f.Get("server")
			if targetServerName == "" {
				targetServerName = lurkcoinName
			}
			err = r.AuthenticateV2(f, targetServerName)
			if err != nil {
				return nil, err
			}

			target := f.Get("target")
			targetServer, ok := r.DbTransaction.GetCachedServer(targetServerName)
			if !ok {
				return nil, errors.New("ERR_SERVERNOTFOUND")
			}

			_, err = r.Server.Pay("", target, targetServer,
				amount, v2IsYes(f.Get("local_currency")), true)
			if err != nil {
				return nil, err
			}
			return "Transaction sent!", nil
		})

	v2Post(router, db, "bal", true,
		func(r *HTTPRequest, _ v2Form) (interface{}, error) {
			return r.Server.GetBalance(), nil
		})

	v2Post(router, db, "history", true,
		func(r *HTTPRequest, f v2Form) (interface{}, error) {
			history := lurkcoin.GetV2History(r.Server.GetSummary(), false)
			if f.Get("json") == "" {
				return strings.Join(history, "\n"), nil
			} else {
				return history, nil
			}
		})

	v2Post(router, db, "exchange_rates", false,
		func(r *HTTPRequest, f v2Form) (interface{}, error) {
			amount, err := lurkcoin.ParseCurrency(f.Get("amount"))
			if err != nil {
				return nil, err
			}

			return lurkcoin.GetExchangeRate(r.Database, f.Get("from"),
				f.Get("to"), amount)
		})

	// A near duplicate of the above endpoint.
	// This doesn't check for authentication
	v2Post(router, db, "get_exchange_rate", false,
		func(r *HTTPRequest, f v2Form) (interface{}, error) {
			amount, err := lurkcoin.ParseCurrency(f.Get("amount"))
			if err != nil {
				return nil, err
			}

			return lurkcoin.GetExchangeRate(r.Database, f.Get("name"),
				f.Get("to"), amount)
		})

	//
	v2Post(router, db, "get_transactions", true,
		func(r *HTTPRequest, f v2Form) (interface{}, error) {
			transactions := r.Server.GetPendingTransactions()
			if f.Get("simple") != "" {
				if len(transactions) == 0 {
					_, exc := r.Server.GetExchangeRate(c1, false)
					return exc, nil
				}
				s := func(n string) string {
					return strings.Replace(n, "|", "/", -1)
				}
				transaction := transactions[0]
				// To support fragile clients (such as versions of the lurkcoin
				// mod that use the /v2 API), "¤" is replaced with "_".
				return fmt.Sprintf("%d|%s|%s|%s",
					transaction.GetLegacyID(),
					s(strings.Replace(transaction.Target, "¤", "_", -1)),
					transaction.ReceivedAmount.RawString(),
					s(transaction.String()),
				), nil
			}
			res := make([][4]interface{}, len(transactions))
			for i, transaction := range transactions {
				res[i] = [4]interface{}{
					transaction.GetLegacyID(),
					strings.Replace(transaction.Target, "¤", "_", -1),
					transaction.ReceivedAmount,
					transaction.String(),
				}
			}
			if v2IsYes(f.Get("as_object")) {
				_, exc := r.Server.GetExchangeRate(c1, false)
				return map[string]interface{}{
					"exchange_rate": json.RawMessage(exc.String()),
					"transactions":  res,
				}, nil
			} else {
				return res, nil
			}
		})

	// lurkcoinV2 silently ignored invalid "amount" values.
	v2Post(router, db, "remove_transactions", true,
		func(r *HTTPRequest, f v2Form) (interface{}, error) {
			amount, err := strconv.Atoi(f.Get("amount"))
			if err != nil || amount < 1 {
				amount = 1
			}
			r.Server.RemoveFirstPendingTransactions(amount)
			return "Done!", nil
		})

	// Exchange rate multipliers don't exist in lurkcoinV3, however something
	// similar can be approximated with target balances.
	v2Post(router, db, "get_exchange_multiplier", true,
		func(r *HTTPRequest, _ v2Form) (interface{}, error) {
			// Fixed exchange rates didn't exist in lurkcoinV2.
			targetBalance := r.Server.GetTargetBalance()
			if targetBalance.IsZero() {
				return 1, nil
			}
			multiplier := new(big.Float).Quo(targetBalance.Float(),
				f500k)
			return json.RawMessage(multiplier.String()), nil
		})

	v2Post(router, db, "set_exchange_multiplier", true,
		func(r *HTTPRequest, f v2Form) (interface{}, error) {
			multiplier, ok := new(big.Float).SetString(f.Get("multiplier"))
			if !ok || multiplier.Cmp(f0) != 1 {
				return nil, errors.New("ERR_INVALIDAMOUNT")
			}
			targetBalanceF := new(big.Float).Mul(multiplier, f500k)
			targetBalance := lurkcoin.CurrencyFromFloat(targetBalanceF)
			ok = r.Server.SetTargetBalance(targetBalance)
			if !ok {
				return nil, errors.New("ERR_INVALIDAMOUNT")
			}
			return "Exchange rate multiplier updated!", nil
		})
}
