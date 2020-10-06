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
	"encoding/json"
	"errors"
	"io"
	"sort"
	"sync"
)

type Database interface {
	// GetServers(serverNames) (servers, ok, badServer)
	// This must atomically get all servers specified, and if one fails free
	// the previous ones and return nil, false, <failed server UID>.
	// NOTE THAT THIS WILL DEADLOCK IF DUPLICATE SERVERs ARE PROVIDED! Use
	// a DatabaseTransaction object to mitigate this issue.
	GetServers([]string) ([]*Server, bool, string)

	// FreeServers(servers, saveChanges)
	// This must atomically free all servers in servers, and if saveChanges is
	// true write any changes to the database.
	FreeServers([]*Server, bool)

	CreateServer(string) (*Server, bool)
	ListServers() []string
	DeleteServer(string) bool
}

// An atomic database transaction.
type DatabaseTransaction struct {
	db      Database
	lock    *sync.Mutex
	servers map[string]*Server
}

// Attempt to use the cache to get servers. Not goroutine-safe.
func (self *DatabaseTransaction) getFromCache(names []string) ([]*Server, bool, string) {
	servers := make([]*Server, len(names))
	for i, name := range names {
		server, exists := self.servers[name]
		if !exists {
			return nil, false, ""
		}
		servers[i] = server
	}

	return servers, true, ""
}

// Get a server. The server will be freed once Finish() or Abort() is called.
func (self *DatabaseTransaction) GetServers(names ...string) ([]*Server, bool, string) {
	self.lock.Lock()
	defer self.lock.Unlock()

	// Ensure that this is the first GetServers() call.
	if self.servers != nil {
		// If GetServers() has been called previously, attempt to use cache.
		servers, ok, badServer := self.getFromCache(names)
		if !ok {
			panic("Multiple calls to GetServers() on DatabaseTransaction.")
		}
		return servers, ok, badServer
	}
	self.servers = make(map[string]*Server)

	// Deduplicate the list
	deduplicated := false
	rawNames := names
	if len(names) > 1 {
		// Search for duplicates
		known := make(map[string]bool, len(names))
		i := 0
		for _, name := range names {
			name = HomogeniseUsername(name)
			if known[name] {
				deduplicated = true
				continue
			}
			names[i] = name
			known[name] = true
			i++
		}

		names = names[:i]
	}

	// Otherwise call GetServer
	servers, ok, badServer := self.db.GetServers(names)
	if ok {
		for _, server := range servers {
			self.servers[server.UID] = server
		}
	}

	// If the list has been deduplicated, call getFromCache().
	if deduplicated && ok {
		return self.getFromCache(rawNames)
	}

	return servers, ok, badServer
}

func (self *DatabaseTransaction) GetOneServer(name string) (server *Server, ok bool) {
	var servers []*Server
	servers, ok, _ = self.GetServers(name)
	if ok {
		server = servers[0]
	}
	return
}

// Get a server already in the cache
func (self *DatabaseTransaction) GetCachedServer(name string) (server *Server, ok bool) {
	name = HomogeniseUsername(name)
	self.lock.Lock()
	defer self.lock.Unlock()
	server, ok = self.servers[name]
	return
}

// Creates a server. This may or may not be able to be reverted with Abort().
func (self *DatabaseTransaction) CreateServer(name string) (*Server, bool) {
	self.lock.Lock()
	defer self.lock.Unlock()

	if self.servers == nil {
		self.servers = make(map[string]*Server)
	}

	name, _ = PasteuriseUsername(name)
	server, ok := self.db.CreateServer(name)
	if ok {
		self.servers[HomogeniseUsername(name)] = server
	}
	return server, ok
}

// Gets a server or creates one if it doesn't exist.
func (self *DatabaseTransaction) GetOrCreateServer(name string) (*Server, bool) {
	servers, ok, _ := self.GetServers(name)
	if !ok {
		return self.CreateServer(name)
	}
	return servers[0], ok
}

// Calls the underlying database's ListServers().
func (self *DatabaseTransaction) ListServers() []string {
	return self.db.ListServers()
}

// Iterate over the database. Server objects are freed after f() returns.
func (self *DatabaseTransaction) ForEach(f func(*Server) error, saveChanges bool) error {
	serverNames := self.ListServers()
	sort.Strings(serverNames)

	// Abort if f() panics.
	defer self.Abort()

	for _, name := range serverNames {
		server, ok := self.GetOneServer(name)

		// If the server has been deleted in the meantime, ignore it.
		if !ok {
			continue
		}

		// If f(server) returns an error then stop iterating.
		err := f(server)
		if err != nil {
			return err
		}

		// Unlock the server (this is the same as calling Finish/Abort).
		self.free(saveChanges)
	}
	return nil
}

func ForEach(db Database, f func(*Server) error, saveChanges bool) error {
	return BeginDbTransaction(db).ForEach(f, saveChanges)
}

func (self *DatabaseTransaction) free(save bool) {
	self.lock.Lock()
	defer self.lock.Unlock()

	if self.servers == nil {
		return
	}

	servers := make([]*Server, 0, len(self.servers))
	for _, server := range self.servers {
		servers = append(servers, server)
	}
	self.db.FreeServers(servers, save)

	self.servers = nil
}

// Commits the changes made to the database.
func (self *DatabaseTransaction) Finish() {
	self.free(true)
}

// Aborts the transaction and discards any changes made. This is a no-op if
// Finish() or Abort() have already been called.
func (self *DatabaseTransaction) Abort() {
	self.free(false)
}

func (self *DatabaseTransaction) GetRawDatabase() Database {
	return self.db
}

// Creates a new DatabaseTransaction object for a database.
func BeginDbTransaction(db Database) *DatabaseTransaction {
	var mutex sync.Mutex
	return &DatabaseTransaction{db, &mutex, nil}
}

func AuthenticateRequest(db Database, username, token string,
	otherServers []string) (bool, *DatabaseTransaction, *Server) {
	// Begin a database transaction.
	tr := BeginDbTransaction(db)

	// Calling tr.GetServers(username, otherServers...) doesn't work
	serverNames := make([]string, len(otherServers)+1)
	serverNames[0] = username
	copy(serverNames[1:], otherServers)

	// Attempt to authenticate the request.
	servers, exists, badServer := tr.GetServers(serverNames...)

	// Get servers before any non-existent server.
	if !exists {
		for i, serverName := range serverNames {
			if badServer == HomogeniseUsername(serverName) {
				serverNames = serverNames[:i]
				break
			}
		}
		if len(serverNames) > 0 {
			tr.Abort()
			servers, exists, _ = tr.GetServers(serverNames...)
		}
	}

	// Check the token.
	if exists && servers[0].CheckToken(token) {
		return true, tr, servers[0]
	}

	// If the authentication failed, abort the transaction and return.
	tr.Abort()
	return false, nil, nil
}

// Backup a database.
func BackupDatabase(db Database, writer io.Writer) error {
	tr := BeginDbTransaction(db)
	defer tr.Abort()

	// Make a list of encoded servers. This uses pointers to reduce copying.
	var encodedServers []*EncodedServer
	tr.ForEach(func(server *Server) error {
		encodedServer := server.Encode()
		encodedServers = append(encodedServers, &encodedServer)
		return nil
	}, false)

	// Nothing was changed, abort the transaction.
	tr.Abort()

	// Save the encoded servers with JSON.
	encoder := json.NewEncoder(writer)
	return encoder.Encode(encodedServers)
}

// Restore a database. This is not atomic and may result in a partially
// restored database.
// TODO: Delete servers that exist in the database but do not exist in the
// backup.
func RestoreDatabase(db Database, reader io.Reader) error {
	var encodedServers []EncodedServer
	decoder := json.NewDecoder(reader)
	err := decoder.Decode(&encodedServers)
	if err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("Extra JSON value")
	}

	tr := BeginDbTransaction(db)
	defer tr.Abort()

	for _, encodedServer := range encodedServers {
		server, ok := tr.GetOrCreateServer(encodedServer.Name)
		if !ok {
			return errors.New("Could not create server.")
		}

		// Overwrite the server
		*server = *encodedServer.Decode()
		server.SetModified()

		// Save
		tr.Finish()
	}
	return nil
}
