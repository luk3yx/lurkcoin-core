//
// lurkcoin configuration
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

package api

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"log"
	"lurkcoin"
	"lurkcoin/databases"
	"net/http"
	"os"
	"strings"
)

type Config struct {
	// The name of this service (for example "lurkcoin"). This is also used as
	// the default server name for the v2 API.
	Name string `yaml:"name"`

	// The address to bind to (optional) and port.
	Address string `yaml:"address"`
	Port    uint16 `yaml:"port"`

	// An optional logfile
	Logfile string `yaml:"logfile"`

	Database struct {
		Type     string            `yaml:"type"`
		Location string            `yaml:"location"`
		Options  map[string]string `yaml:"options"`
	} `yaml:"database"`

	// TLS
	TLS struct {
		Enable   bool   `yaml:"enable"`
		CertFile string `yaml:"cert_file"`
		KeyFile  string `yaml:"key_file"`
	} `yaml:"tls"`

	// Admin pages
	AdminPages struct {
		Enable bool              `yaml:"enable"`
		Users  AdminLoginDetails `yaml:"users"`
	} `yaml:"admin_pages"`

	// HTTP redirects
	Redirects map[string]string `yaml:"redirects"`

	// The minimum HTTPS API version to support.
	MinAPIVersion uint8 `yaml:"min_api_version"`
}

func LoadConfig(filename string) (*Config, error) {
	f, err := os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var config Config
	decoder := yaml.NewDecoder(f)
	decoder.SetStrict(true)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	if config.Name == "lurkcoin" {
		log.Println("Warning: The selected server name already exists!")
	}
	return &config, nil
}

func OpenDatabase(config *Config) (lurkcoin.Database, error) {
	return databases.OpenDatabase(
		config.Database.Type,
		config.Database.Location,
		config.Database.Options,
	)
}

func StartServer(config *Config) {
	lurkcoin.SeedPRNG()
	lurkcoin.PrintASCIIArt()
	log.Printf("Supported database types: %s",
		strings.Join(databases.GetSupportedDatabaseTypes(), ", "))
	db, err := OpenDatabase(config)
	if err != nil {
		log.Fatal(err)
	}

	router := MakeHTTPRouter(db, config)

	address := fmt.Sprintf("%s:%d", config.Address, config.Port)
	urlAddress := address
	if config.Address == "" {
		urlAddress = "[::]" + urlAddress
	}
	if config.TLS.Enable {
		log.Printf("Starting server on https://%s/", urlAddress)
	} else {
		log.Printf("Starting server on http://%s/", urlAddress)
	}

	if config.Logfile != "" {
		f, err := os.OpenFile(
			config.Logfile,
			os.O_WRONLY|os.O_APPEND|os.O_CREATE,
			0600,
		)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		log.Printf("Using logfile %#v.", config.Logfile)
		log.SetOutput(f)
	}

	server := &http.Server{Addr: address, Handler: router}

	// My laptop doesn't work nicely with Keep-Alive.
	server.SetKeepAlivesEnabled(false)

	if config.TLS.Enable {
		err = server.ListenAndServeTLS(config.TLS.CertFile, config.TLS.KeyFile)
	} else {
		err = server.ListenAndServe()
	}

	log.Fatal(err)
}
