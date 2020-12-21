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

import (
	crypto_rand "crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"math"
	"math/big"
	"math/rand"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

const SYMBOL = "¤"
const VERSION = "3.0.8"

// Note that public source code is required by the AGPL
const SOURCE_URL = "https://github.com/luk3yx/lurkcoin-core"
const REPORT_SECURITY = "https://gitlab.com/luk3yx/lurkcoin-core/-/issues/new"

// Copyrights should be separated by newlines
const COPYRIGHT = "Copyright © 2020 by luk3yx"

func PrintASCIIArt() {
	log.Print(`/\___/\    _            _             _`)
	log.Print(`\  _  /   | |_   _ _ __| | _____ ___ (_)_ __`)
	log.Print(`| (_) |   | | | | | '__| |/ / __/ _ \| | '_ \`)
	log.Print(`/ ___ \   | | |_| | |  |   < (_| (_) | | | | |`)
	log.Print(`\/   \/   |_|\__,_|_|  |_|\_\___\___/|_|_| |_|`)
	log.Print()
	log.Printf("Version %s", VERSION)
}

var c0 Currency = CurrencyFromInt64(0)
var invalid_uid = regexp.MustCompile(`[^a-z0-9\_]`)

func HomogeniseUsername(username string) string {
	username = strings.ToLower(username)
	username = strings.Replace(username, " ", "", -1)
	return invalid_uid.ReplaceAllLiteralString(username, "_")
}

// Remove control characters and leading+trailing whitespace from a username.
// HomogeniseUsername(PasteuriseUsername(username)) should always equal
// HomogeniseUsername(username).
func PasteuriseUsername(username string) (res string, runeCount int) {
	res = strings.Map(func(r rune) rune {
		runeCount += 1
		if unicode.IsGraphic(r) {
			return r
		}
		return '�'
	}, strings.Trim(username, " "))
	return
}

// Gets a random uint64 with crypto/rand, casts it to an int64 and feeds it to
// rand.Seed().
func SeedPRNG() {
	max := new(big.Int).SetUint64(math.MaxUint64)
	res, err := crypto_rand.Int(crypto_rand.Reader, max)
	if err != nil {
		panic(err)
	}
	rand.Seed(int64(res.Uint64()))
}

// Generate a secure random API token. This will probably be around 171
// characters long.
func GenerateToken() string {
	// Get 128 random bytes (1024 bits).
	raw := make([]byte, 128)
	_, err := crypto_rand.Read(raw)
	if err != nil {
		panic(err)
	}

	// Encode it with base64.RawURLEncoding
	var builder strings.Builder
	encoder := base64.NewEncoder(base64.RawURLEncoding, &builder)
	encoder.Write(raw)
	encoder.Close()

	// Return the string
	return builder.String()
}

// Validate a webhook URL, returns the actual URL that should be used and a
// boolean indicating success.
func ValidateWebhookURL(rawURL string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false
	}
	path := u.Path

	// Paths always end in /lurkcoin currently.
	if !strings.HasSuffix(path, "/lurkcoin") {
		if !strings.HasSuffix(path, "/") {
			path += "/"
		}
		path += "lurkcoin"
	}

	// Create a new URL object without extra parameters.
	safeURL := &url.URL{Scheme: u.Scheme, Host: u.Host, Path: path}
	return safeURL.String(), true
}

// Get an exchange rate between two servers
func GetExchangeRate(db Database, source, target string, amount Currency) (Currency, error) {
	tr := BeginDbTransaction(db)
	defer tr.Abort()

	source = HomogeniseUsername(source)
	target = HomogeniseUsername(target)
	if source == target {
		return amount, nil
	}

	if source != "" {
		sourceServer, ok := tr.GetOneServer(source)
		if !ok {
			return c0, errors.New("ERR_SOURCESERVERNOTFOUND")
		}
		amount, _ = sourceServer.GetExchangeRate(amount, true)

		// Abort the transaction now to get the target server
		tr.Abort()
	}
	if target != "" {
		targetServer, ok := tr.GetOneServer(target)
		if !ok {
			return c0, errors.New("ERR_TARGETSERVERNOTFOUND")
		}
		amount, _ = targetServer.GetExchangeRate(amount, false)
	}
	return amount, nil
}

// A Python-ish repr()
func repr(raw string) string {
	res := fmt.Sprintf("%q", raw)
	if strings.Count(res, `"`) == 2 && !strings.Contains(res, "'") {
		return "'" + res[1:len(res)-1] + "'"
	}
	return res
}

// A helper used by both lurkcoin/api and lurkcoin/databases.
func GetV2History(summary Summary, appendID bool) (h []string) {
	balance := summary.Bal
	// This intentionally adds transactions the server sends to itself twice.
	for _, transaction := range summary.History {
		var suffix string
		if appendID {
			suffix = " [" + transaction.ID + "]"
		}
		if transaction.TargetServer == summary.Name {
			amount_s := transaction.Amount.DeltaString()
			if transaction.SourceServer == "" && transaction.Target == "" {
				h = append(h, fmt.Sprintf("%s: %s - %s%s", balance.String(),
					amount_s, transaction.Source, suffix))
			} else {
				h = append(h, fmt.Sprintf(
					"%s: %s - Transaction from %s to %s.%s",
					balance.String(),
					amount_s,
					repr(transaction.SourceServer),
					repr(transaction.Target),
					suffix,
				))
			}

			// Addition and subtraction are swapped because most recent
			// transactions are first and we start with the current balance.
			balance = balance.Sub(transaction.Amount)
		}
		if transaction.SourceServer == summary.Name {
			amount_s := transaction.Amount.Neg().DeltaString()
			h = append(h, fmt.Sprintf("%s: %s - Transaction to %s on %s.%s",
				balance.String(), amount_s, repr(transaction.Target),
				repr(transaction.TargetServer), suffix))
			balance = balance.Add(transaction.Amount)
		}
	}
	if len(h) < 10 {
		h = append(h, c0.String()+": Account created")
	}
	return
}

// Returns true if a == b in a constant time. Note that this will however leak
// string lengths.
func ConstantTimeCompare(a, b string) bool {
	// Comparing the length isn't strictly necessary in Go 1.4+, however is
	// done anyway.
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) != 0
}
