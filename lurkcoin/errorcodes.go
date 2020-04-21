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

package lurkcoin

// Error codes
var errorCodes = map[string]string{
	"ERR_INVALIDLOGIN":   `Invalid login!`,
	"ERR_INVALIDREQUEST": `Invalid request.`,
	"ERR_PAYLOADTOOLARGE": `Request body too large. You may send a maximum ` +
		`of 4096 bytes.`,

	"ERR_SERVERNOTFOUND":   `Server not found!`,
	"ERR_INVALIDAMOUNT":    `Invalid number!`,
	"ERR_CANNOTPAYNOTHING": `You cannot pay someone ` + SYMBOL + `0.00!`,
	"ERR_CANNOTAFFORD":     `You cannot afford to do that!`,

	`ERR_SOURCEUSERNAMETOOLONG`: `The source username is too long!`,
	`ERR_USERNAMETOOLONG`:       `The target username is too long!`,

	// It might be possible to reword these descriptions without breaking things
	"ERR_SOURCESERVERNOTFOUND": `The "from" server does not exist!`,
	"ERR_TARGETSERVERNOTFOUND": `The "to" server does not exist!`,
	"ERR_TRANSACTIONLIMIT":     `The amount you specified exceeds the max spend!`,
}

func LookupError(code string) (string, string, int) {
	msg, exists := errorCodes[code]
	if exists {
		var httpCode int
		switch code {
		case "ERR_INVALIDLOGIN":
			httpCode = 401
		case "ERR_PAYLOADTOOLARGE":
			httpCode = 413
		default:
			httpCode = 400
		}
		return code, msg, httpCode
	} else {
		return "ERR_INTERNALERROR", "Internal error!", 500
	}
}
