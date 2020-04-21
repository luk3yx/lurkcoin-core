//
// lurkcoin database using bbolt: https://github.com/etcd-io/bbolt.
// This is the recommended database format for lurkcoin.
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

// +build !lurkcoin.disablebbolt,!wasm lurkcoin.enablebbolt

package databases

import (
	"bytes"
	"encoding/gob"
	"errors"
	"lurkcoin"

	bolt "github.com/etcd-io/bbolt"
)

type boltDatabase struct {
	db     *bolt.DB
	dblock genericDbLock
}

func (self *boltDatabase) GetServers(names []string) ([]*lurkcoin.Server, bool, string) {
	// Acquire locks
	names = self.dblock.Lock(names)

	// Unlock if there is an error
	ok := false
	defer func() {
		if !ok {
			self.dblock.UnlockIDs(names)
		}
	}()

	res := make([]*lurkcoin.Server, len(names))
	var serverName string
	err := self.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("lurkcoin"))
		if bucket == nil {
			if len(names) > 0 {
				serverName = names[0]
			}
			return errors.New("Bucket does not exist")
		}

		for i, name := range names {
			raw := bucket.Get([]byte(name))
			if len(raw) == 0 {
				serverName = name
				return errors.New("ERR_SERVERNOTFOUND")
			}
			decoder := gob.NewDecoder(bytes.NewBuffer(raw))
			var encodedServer lurkcoin.EncodedServer
			if err := decoder.Decode(&encodedServer); err != nil {
				return err
			}
			res[i] = encodedServer.Decode()
		}

		return nil
	})

	if err == nil {
		ok = true
		return res, true, serverName
	} else {
		return nil, false, serverName
	}
}

func (self *boltDatabase) FreeServers(servers []*lurkcoin.Server, save bool) {
	defer self.dblock.Unlock(servers)
	if !save {
		return
	}
	err := self.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("lurkcoin"))
		if err != nil {
			return err
		}
		for _, server := range servers {
			if !server.IsModified() {
				continue
			}
			var buf bytes.Buffer
			encoder := gob.NewEncoder(&buf)
			encoder.Encode(server.Encode())
			bucket.Put([]byte(server.UID), buf.Bytes())
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
}

// Creates a server. The server is not saved until FreeServer() is called.
func (self *boltDatabase) CreateServer(name string) (*lurkcoin.Server, bool) {
	ids := self.dblock.Lock([]string{name})

	err := self.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("lurkcoin"))
		if bucket != nil && len(bucket.Get([]byte(ids[0]))) != 0 {
			return errors.New("")
		}
		return nil
	})
	if err != nil {
		self.dblock.UnlockIDs(ids)
		return nil, false
	}

	server := lurkcoin.NewServer(name)
	return server, true
}

func (self *boltDatabase) ListServers() (res []string) {
	self.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("lurkcoin"))
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(k, v []byte) error {
			res = append(res, string(k))
			return nil
		})
	})
	return
}

func BoltDatabase(file string, _ map[string]string) (lurkcoin.Database, error) {
	db, err := bolt.Open(file, 0600, nil)
	if err != nil {
		return nil, err
	}
	return &boltDatabase{db, newGenericDbLock()}, nil
}

func init() {
	RegisterDatabaseType("bolt", BoltDatabase)
	RegisterDatabaseType("bbolt", BoltDatabase)
}
