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

package databases

import (
	"fmt"
	"lurkcoin"
	"sort"
	"strings"
	"sync"
)

type databaseFactory func(location string, options map[string]string) (lurkcoin.Database, error)

var databaseTypes = make(map[string]databaseFactory)

// Registers a new database type.
// WARNING: This function is not goroutine-safe and should probably only be
// called from init().
func RegisterDatabaseType(name string, f databaseFactory) {
	databaseTypes[strings.ToLower(name)] = f
}

// Opens a database. The options parameter can be nil.
func OpenDatabase(dbType, location string, options map[string]string) (lurkcoin.Database, error) {
	f, exists := databaseTypes[strings.ToLower(dbType)]
	if exists {
		return f(location, options)
	}
	return nil, fmt.Errorf("Unknown database type: %v.", dbType)
}

func GetSupportedDatabaseTypes() []string {
	res := make([]string, 0, len(databaseTypes))
	for dbType := range databaseTypes {
		res = append(res, dbType)
	}
	sort.Strings(res)
	return res
}

// Generic database lock
type genericDbLock struct {
	lock  *sync.Mutex
	locks map[string]*sync.Mutex
}

// Locks servers and returns a list of homogenised server names.
// It would probably be more efficient to just use one larger lock.
func (self *genericDbLock) Lock(names []string) []string {
	ids := make([]string, len(names))
	for i, name := range names {
		ids[i] = lurkcoin.HomogeniseUsername(name)
	}

	// Ensure none of the servers are locked.
	self.lock.Lock()
	ok := false
	for !ok {
		ok = true
		for _, name := range ids {
			cachedServerLock, exists := self.locks[name]
			if !exists {
				continue
			}

			ok = false
			self.lock.Unlock()
			cachedServerLock.Lock()
			cachedServerLock.Unlock()
			self.lock.Lock()
			break
		}
	}

	defer self.lock.Unlock()

	// Create locks so the above code does not have to make use of polling.
	for _, name := range ids {
		var lock sync.Mutex
		lock.Lock()
		self.locks[name] = &lock
	}

	return ids
}

// Unlocks
func (self *genericDbLock) UnlockIDs(ids []string) {
	self.lock.Lock()
	defer self.lock.Unlock()
	for _, id := range ids {
		self.locks[id].Unlock()
		delete(self.locks, id)
	}
}

func (self *genericDbLock) Unlock(servers []*lurkcoin.Server) {
	self.lock.Lock()
	defer self.lock.Unlock()
	for _, server := range servers {
		self.locks[server.UID].Unlock()
		delete(self.locks, server.UID)
	}
}

func newGenericDbLock() genericDbLock {
	return genericDbLock{new(sync.Mutex), make(map[string]*sync.Mutex)}
}
