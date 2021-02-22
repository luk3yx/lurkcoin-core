//
// lurkcoin plaintext database
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

// +build !lurkcoin.disableplaintextdb

package databases

import (
	"encoding/json"
	"github.com/luk3yx/lurkcoin-core/lurkcoin"
	"io/ioutil"
	"os"
	"path"
	"sync"
)

type plaintextDatabase struct {
	db       map[string]*lurkcoin.EncodedServer
	location string
	dblock   genericDbLock
	lock     *sync.RWMutex
}

func (self *plaintextDatabase) GetServers(names []string) ([]*lurkcoin.Server, bool, string) {
	// Acquire locks
	names = self.dblock.Lock(names)

	// Unlock if there is an error
	ok := false
	defer func() {
		if !ok {
			self.dblock.UnlockIDs(names)
		}
	}()

	self.lock.RLock()
	defer self.lock.RUnlock()

	servers := make([]*lurkcoin.Server, 0, len(names))
	for _, name := range names {
		encodedServer, exists := self.db[name]
		if !exists {
			return nil, false, name
		}
		servers = append(servers, encodedServer.Decode())
	}

	ok = true
	return servers, ok, ""
}

func (self *plaintextDatabase) save() {
	f, err := ioutil.TempFile(path.Dir(self.location), ".tmp")
	if err != nil {
		panic(err)
	}
	fn := f.Name()
	defer func() {
		if fn != "" {
			f.Close()
			os.Remove(fn)
		}
	}()

	encodedServers := make([]*lurkcoin.EncodedServer, 0, len(self.db))
	for _, encodedServer := range self.db {
		encodedServers = append(encodedServers, encodedServer)
	}
	encoder := json.NewEncoder(f)
	err = encoder.Encode(encodedServers)
	if err != nil {
		panic(err)
	}

	f.Close()
	err = os.Rename(fn, self.location)
	if err != nil {
		panic(err)
	}
}

func (self *plaintextDatabase) FreeServers(servers []*lurkcoin.Server, save bool) {
	self.lock.Lock()
	defer self.lock.Unlock()
	self.dblock.Unlock(servers)

	if !save {
		return
	}

	modified := false
	for _, server := range servers {
		if server.IsModified() {
			modified = true
			encodedServer := server.Encode()
			self.db[server.UID] = &encodedServer
		}
	}

	if modified {
		self.save()
	}
}

func (self *plaintextDatabase) CreateServer(name string) (*lurkcoin.Server, bool) {
	ids := self.dblock.Lock([]string{name})
	id := ids[0]

	self.lock.Lock()
	defer self.lock.Unlock()
	_, exists := self.db[id]
	if exists {
		self.dblock.UnlockIDs(ids)
		return nil, false
	}

	return lurkcoin.NewServer(name), true
}

func (self *plaintextDatabase) ListServers() []string {
	self.lock.Lock()
	defer self.lock.Unlock()
	res := make([]string, len(self.db))
	i := 0
	for k := range self.db {
		res[i] = k
		i++
	}
	return res
}

func (self *plaintextDatabase) DeleteServer(name string) (exists bool) {
	ids := self.dblock.Lock([]string{name})
	defer self.dblock.UnlockIDs(ids)
	id := ids[0]
	_, exists = self.db[id]
	if exists {
		delete(self.db, id)
		self.save()
	}
	return
}

func PlaintextDatabase(location string, _ map[string]string) (lurkcoin.Database, error) {
	db := &plaintextDatabase{
		make(map[string]*lurkcoin.EncodedServer),
		location,
		newGenericDbLock(),
		new(sync.RWMutex),
	}
	f, err := os.OpenFile(location, os.O_RDONLY, 0)
	if err == nil {
		err = lurkcoin.RestoreDatabase(db, f)
		if err != nil {
			return nil, err
		}
	}
	return db, nil
}

func init() {
	RegisterDatabaseType("plaintext", PlaintextDatabase)
}
