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
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Most mutable fields in Server are private to prevent race conditions.
// Note that pendingTransactions has to be ordered to retain lurkcoinV2
// compatibility. If compatibiltiy is ever dropped, it could possibly become a
// map[string]Transaction to improve the efficiency of delete operations.
type Server struct {
	UID                 string
	Name                string
	balance             Currency
	targetBalance       Currency
	history             []Transaction
	pendingTransactions []Transaction
	token               string
	WebhookURL          string
	lock                *sync.RWMutex
	modified            bool
}

type ServerCollection interface {
	GetServer(name string) *Server
}

var MaxTargetBalance = CurrencyFromInt64(500000000)

func (self *Server) GetBalance() Currency {
	self.lock.RLock()
	defer self.lock.RUnlock()
	return self.balance
}

func (self *Server) GetTargetBalance() Currency {
	self.lock.RLock()
	defer self.lock.RUnlock()
	return self.targetBalance
}

// Changes the user's balance, returns false if the user does not have enough
// money. This is an atomic operation, changing the balance manually is not
// recommended.
func (self *Server) ChangeBal(num Currency) bool {
	self.lock.Lock()
	defer self.lock.Unlock()
	new_balance := self.balance.Add(num)
	if new_balance.LtZero() {
		return false
	}
	self.balance = new_balance
	self.modified = true
	return true
}

// Gets the server's history. The slice returned can be modified, however the
// transaction objects should not be.
func (self *Server) GetHistory() []Transaction {
	self.lock.RLock()
	defer self.lock.RUnlock()
	res := make([]Transaction, len(self.history))
	copy(res, self.history)
	return res
}

var webhookClient = &http.Client{Timeout: time.Second * 5}

func (self *Server) AddToHistory(transaction Transaction) {
	self.lock.Lock()
	defer self.lock.Unlock()
	self.modified = true

	// Prepend transaction to self.history
	// https://stackoverflow.com/a/53737602
	if len(self.history) < 10 {
		// Only increase the length of the slice if it is shorter than 10
		// elements long, meaning the transaction history cannot be longer
		// than 10 elements.
		self.history = append(self.history, Transaction{})
	}
	copy(self.history[1:], self.history)
	self.history[0] = transaction

	if self.Name != transaction.TargetServer || transaction.Target == "" {
		return
	}

	// Add to pending transactions.
	self.pendingTransactions = append(self.pendingTransactions, transaction)

	// Validate the webhook URL (if any).
	if self.WebhookURL == "" {
		return
	}

	// Send a request to the webhook (in a separate goroutine so it doesn't
	// block anything).
	go func(webhookURL string) {
		url, ok := ValidateWebhookURL(webhookURL)
		if !ok {
			return
		}
		reader := strings.NewReader(`{"version": 0}`)
		req, err := http.NewRequest("POST", url, reader)
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "lurkcoin/3.0")
		res, err := webhookClient.Do(req)
		if err == nil {
			res.Body.Close()
		}
	}(self.WebhookURL)
}

// Get a list of pending transactions, similar to GetHistory().
func (self *Server) GetPendingTransactions() []Transaction {
	self.lock.RLock()
	defer self.lock.RUnlock()
	res := make([]Transaction, len(self.pendingTransactions))
	copy(res, self.pendingTransactions)
	return res
}

// Returns true if the server has pending transactions.
func (self *Server) HasPendingTransactions() bool {
	self.lock.RLock()
	defer self.lock.RUnlock()
	return len(self.pendingTransactions) > 0
}

func (self *Server) removeAndReturnPendingTransaction(id string) *Transaction {
	self.lock.Lock()
	defer self.lock.Unlock()
	for i, transaction := range self.pendingTransactions {
		if transaction.ID == id {
			// Although Currency objects are not themselves pointers, they
			// contain a pointer to a big.Int object.
			l := len(self.pendingTransactions) - 1
			if i < l {
				copy(self.pendingTransactions[i:],
					self.pendingTransactions[i+1:])
			}
			self.pendingTransactions[l] = Transaction{}
			self.pendingTransactions = self.pendingTransactions[:l]
			self.modified = true
			return &transaction
		}
	}
	return nil
}

// Remove a pending transaction given its ID.
func (self *Server) RemovePendingTransaction(id string) {
	self.removeAndReturnPendingTransaction(id)
}

// Reject (and possibly revert) a pending transaction.
func (self *Server) RejectPendingTransaction(id string,
	tr *DatabaseTransaction) {
	if tr == nil {
		panic("nil *DatabaseTransaction passed to RejectPendingTransaction().")
	}

	// Get the transaction and ensure
	transaction := self.removeAndReturnPendingTransaction(id)
	if transaction == nil || !transaction.Revertable {
		return
	}

	// Defer to a goroutine to prevent deadlocks
	// TODO: Do this in the current goroutine
	db := tr.GetRawDatabase()
	currentUID := self.UID
	go func() {
		// Get the current server (the existing object is now invalid) and the
		// source server.
		tr := BeginDbTransaction(db)
		defer tr.Abort()

		servers, ok, _ := tr.GetServers(currentUID, transaction.SourceServer)
		if !ok {
			return
		}

		// To try and prevent exploits, the received amount is used and exchange
		// rates are re-calculated.
		// Note that the source and target get flipped here.
		servers[0].Pay(transaction.Target, transaction.Source, servers[1],
			transaction.ReceivedAmount, true, false)
		tr.Finish()
	}()
}

// Remove the first <amount> pending transactions.
// This is here to support lurkcoinV2 and probably shouldn't be used outside of
// that.
func (self *Server) RemoveFirstPendingTransactions(amount int) {
	self.lock.Lock()
	defer self.lock.Unlock()

	if amount < 1 {
		return
	}

	self.modified = true
	l := len(self.pendingTransactions)
	copy(self.pendingTransactions, self.pendingTransactions[amount:])
	for i := l - amount; i < l; i++ {
		self.pendingTransactions[i] = Transaction{}
	}
	self.pendingTransactions = self.pendingTransactions[:l-amount]
}

// Sets the target balance.
func (self *Server) SetTargetBalance(targetBalance Currency) bool {
	if targetBalance.LtZero() || targetBalance.Gt(MaxTargetBalance) {
		return false
	}

	self.lock.Lock()
	defer self.lock.Unlock()
	self.modified = true
	self.targetBalance = targetBalance
	return true
}

// Validates and sets a webhook URL.
func (self *Server) SetWebhookURL(webhookURL string) (ok bool) {
	var safeURL string
	if webhookURL == "" {
		// Allow clearing the webhook URL
		safeURL, ok = "", true
	} else {
		// This calls ValidateWebhookURL() so that the URL does not change if
		// the rules are relaxed in the future.
		safeURL, ok = ValidateWebhookURL(webhookURL)
	}

	if !ok {
		return
	}

	self.lock.Lock()
	defer self.lock.Unlock()
	self.modified = true
	self.WebhookURL = safeURL
	return
}

// Gets the exchange rate.
// GetExchangeRate(<lurkcoins>, false) → <local currency>
// GetExchangeRate(<local currency>, true) → <lurkcoins>
var f2 = big.NewFloat(2)

// Exchange rate calculations are horrible at the moment, however they work
// (at least I think they work).
func (self *Server) GetExchangeRate(amount Currency, toLurkcoin bool) (Currency,
	*big.Float) {
	self.lock.RLock()
	defer self.lock.RUnlock()

	// Do nothing if the amount is 0 or fixed exchange rates are enabled.
	if amount.IsZero() || self.targetBalance.IsZero() {
		return amount, big.NewFloat(1)
	}

	// bal = max(self.balance, 0.01)
	bal := self.balance
	if !bal.GtZero() {
		bal = CurrencyFromString("0.01")
	}

	// base_exchange = self.TargetBal / bal
	base_exchange := self.targetBalance.Div(bal)

	// To lurkcoin: adj_bal = bal - amount / base_exchange
	// From lurkcoin: adj_bal = bal + amount
	var adj_bal Currency
	if toLurkcoin {
		adj_bal = bal.Sub(CurrencyFromFloat(new(big.Float).Quo(amount.Float(),
			base_exchange)))
	} else {
		adj_bal = bal.Add(amount)
	}

	// Calculate the "pre-emptive" exchange rate and average the two.
	preemptive := new(big.Float).Add(base_exchange,
		self.targetBalance.Div(adj_bal))
	exchange := new(big.Float).Quo(preemptive, f2)

	// Multiply (or divide) the exchange rate and the amount
	res := new(big.Float)
	if toLurkcoin {
		res.Quo(amount.Float(), exchange)
	} else {
		res.Mul(amount.Float(), exchange)
	}
	return CurrencyFromFloat(res), exchange
}

// "Encoded" servers that have all their values public
type EncodedServer struct {
	// A version number for breaking changes, because of the way gob works this
	// can be upgraded to a uint16/uint32 at a later time.
	Version uint8 `json:"version"`

	// The server name (not passed through HomogeniseUsername)
	Name string `json:"name"`

	// The balance in integer form where 1234 is ¤12.34.
	Balance *big.Int `json:"balance"`

	// The target balance in the same format as the above balance.
	TargetBalance *big.Int `json:"target_balance"`

	// Other values
	History             []Transaction `json:"history"`
	PendingTransactions []Transaction `json:"pending_transactions"`
	Token               string        `json:"token"`
	WebhookURL          string        `json:"webhook_url"`
}

func (self *Server) IsModified() bool {
	self.lock.RLock()
	defer self.lock.RUnlock()
	return self.modified
}

func (self *Server) SetModified() {
	self.lock.Lock()
	defer self.lock.Unlock()
	self.modified = true
}

func (self *Server) Encode() EncodedServer {
	self.lock.RLock()
	defer self.lock.RUnlock()

	history := make([]Transaction, len(self.history))
	copy(history, self.history)
	pendingTransactions := make([]Transaction, len(self.pendingTransactions))
	copy(pendingTransactions, self.pendingTransactions)
	return EncodedServer{0, self.Name, self.balance.Int(),
		self.targetBalance.Int(), history, pendingTransactions, self.token,
		self.WebhookURL}
}

func (self *EncodedServer) Decode() *Server {
	if self.Version > 0 {
		panic("Unrecognised EncodedServer version!")
	}
	if self.Balance == nil || self.TargetBalance == nil {
		panic("Invalid EncodedServer passed to EncodedServer.Decode()!")
	}

	// Convert Balance and TargetBalance to Currency.
	balance := CurrencyFromInt(self.Balance)
	targetBalance := CurrencyFromInt(self.TargetBalance)

	// Copy History and PendingTransactions.
	history := make([]Transaction, len(self.History))
	copy(history, self.History)
	pendingTransactions := make([]Transaction, len(self.PendingTransactions))
	copy(pendingTransactions, self.PendingTransactions)

	return &Server{HomogeniseUsername(self.Name), self.Name, balance,
		targetBalance, history, pendingTransactions, self.Token,
		self.WebhookURL, new(sync.RWMutex), false}
}

// Summaries
type Summary struct {
	UID           string        `json:"uid"`
	Name          string        `json:"name"`
	Bal           Currency      `json:"bal"`
	Balance       string        `json:"balance"`
	History       []Transaction `json:"history"`
	InterestRate  float64       `json:"interest_rate"`
	TargetBalance Currency      `json:"target_balance"`
}

func (self *Server) GetSummary() Summary {
	self.lock.RLock()
	defer self.lock.RUnlock()
	return Summary{self.UID, self.Name, self.balance, self.balance.String(),
		self.GetHistory(), 0, self.targetBalance}
}

// Check an API token.
// WARNING: This may leak the length of the stored token, however that is
// probably already deducible by inspecting GenerateToken().
func (self *Server) CheckToken(token string) bool {
	if self.token == "" {
		return false
	}

	return ConstantTimeCompare(self.token, token)
}

// Make a new server
// The default target balance is currently ¤500,000.
const DefaultTargetBalance int64 = 500000

func NewServer(name string) *Server {
	var server EncodedServer
	server.Version = 0
	server.Name = name
	server.Balance = new(big.Int).SetInt64(0)
	server.TargetBalance = new(big.Int).SetInt64(DefaultTargetBalance * 100)
	server.Token = GenerateToken()

	res := server.Decode()
	res.SetModified()
	return res
}
