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
	"errors"
	"log"
)

// The transaction limit, currently 1e+11 so clients that parse JSON numbers as
// 64-bit floats won't run into issues.
var transactionLimit Currency = CurrencyFromInt64(100000000000)

// Sends a payment.
func (sourceServer *Server) Pay(source, target string,
	targetServer *Server, sentAmount Currency, localCurrency bool,
	revertable bool) (*Transaction, error) {

	// Ensure the source and target usernames aren't too long.
	var length int
	source, length = PasteuriseUsername(source)
	if length > 48 {
		return nil, errors.New("ERR_SOURCEUSERNAMETOOLONG")
	}
	target, length = PasteuriseUsername(target)
	if length > 48 {
		return nil, errors.New("ERR_USERNAMETOOLONG")
	}

	// Get the amount being sent in lurkcoins
	var amount Currency
	if localCurrency {
		amount, _ = sourceServer.GetExchangeRate(sentAmount, true)
	} else {
		amount = sentAmount
	}

	// No stealing
	if !sentAmount.GtZero() || !amount.GtZero() {
		return nil, errors.New("ERR_INVALIDAMOUNT")
	}

	if sentAmount.Gt(transactionLimit) || amount.Gt(transactionLimit) {
		return nil, errors.New("ERR_TRANSACTIONLIMIT")
	}

	// Remove the amount
	success := sourceServer.ChangeBal(amount.Neg())
	if !success {
		return nil, errors.New("ERR_CANNOTAFFORD")
	}

	receivedAmount, _ := targetServer.GetExchangeRate(amount, false)

	if !receivedAmount.GtZero() {
		return nil, errors.New("ERR_CANNOTPAYNOTHING")
	}

	if receivedAmount.Gt(transactionLimit) {
		return nil, errors.New("ERR_TRANSACTIONLIMIT")
	}

	success = targetServer.ChangeBal(amount)

	// This should always be true
	if !success {
		// Revert the previous balance change before returning
		sourceServer.ChangeBal(amount)
		return nil, errors.New("ERR_INTERNALERROR")
	}

	transaction := MakeTransaction(source, sourceServer.Name, target,
		targetServer.Name, amount, sentAmount, receivedAmount)
	if revertable {
		transaction.Revertable = true
	}

	// Add the transaction to the history
	if sourceServer != targetServer {
		sourceServer.AddToHistory(transaction)
	}
	targetServer.AddToHistory(transaction)

	// Log the transaction
	log.Print(transaction)

	return &transaction, nil
}
