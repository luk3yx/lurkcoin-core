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
	"fmt"
	"math/big"
	"math/rand"
	"sync"
	"time"
)

type Transaction struct {
	ID             string   `json:"id"`
	Source         string   `json:"source"`
	SourceServer   string   `json:"source_server"`
	Target         string   `json:"target"`
	TargetServer   string   `json:"target_server"`
	Amount         Currency `json:"amount"`
	SentAmount     Currency `json:"sent_amount"`
	ReceivedAmount Currency `json:"received_amount"`
	Time           int64    `json:"time"`

	// If true lurkcoin will attempt to revert the transaction if it is
	// rejected. The transaction can still be rejected if this is false.
	Revertable bool `json:"revertable"`
}

func (self Transaction) String() string {
	return fmt.Sprintf("[%s] %s (sent %s, received %s) - Transaction from %q"+
		" on %q to %q on %q.", self.ID, self.Amount,
		self.SentAmount.RawString(), self.ReceivedAmount.RawString(),
		self.Source, self.SourceServer, self.Target, self.TargetServer)
}

// Get a time.Time object from the transaction's Time attribute.
func (self Transaction) GetTime() time.Time {
	return time.Unix(self.Time, 0)
}

// Get the legacy ID
var max_legacy_id *big.Int = big.NewInt(9999999)

func (self *Transaction) GetLegacyID() int32 {
	raw := new(big.Int)
	raw.Mod(new(big.Int).SetBytes([]byte(self.ID)), max_legacy_id)

	// Because 10,000,000 (max_legacy_id + 1) can fit into int64/int32,
	// overflows are not an issue here.
	return int32(raw.Int64()) + 1
}

// Generate transaction IDs
var mutex = new(sync.Mutex)
var lastTime int64 = -1
var previouslyGenerated map[uint32]bool

func GenerateTransactionID() (string, int64) {
	mutex.Lock()
	defer mutex.Unlock()

	// Ensure too many transaction IDs haven't been generated.
	if len(previouslyGenerated) > 1048576 {
		// Uh-oh (more than 1 million transactions in a single second)
		time.Sleep(1 * time.Second)
	}

	t := time.Now().Unix()
	if t > lastTime {
		previouslyGenerated = make(map[uint32]bool)
	}

	var id uint32
	var exists bool = true
	for exists {
		id = rand.Uint32()
		_, exists = previouslyGenerated[id]
	}
	previouslyGenerated[id] = true

	return fmt.Sprintf("T%X-%08X", t, id), t
}

func MakeTransaction(source, sourceServer, target, targetServer string,
	amount, sentAmount, receivedAmount Currency) Transaction {
	id, time := GenerateTransactionID()
	return Transaction{id, source, sourceServer, target, targetServer, amount,
		sentAmount, receivedAmount, time, false}
}
