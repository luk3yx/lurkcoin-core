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
	"io/ioutil"
	"log"
	"lurkcoin"
	"lurkcoin/databases"
	"net"
	"net/http"
	"os"
	"strings"
)

type Config struct {
	// The name of this service (for example "lurkcoin"). This is also used as
	// the default server name for the v2 API.
	Name string `yaml:"name"`

	// The network protocol to use when binding to the socket. Defaults to
	// "tcp", can be set to "unix" for example.
	NetworkProtocol string `yaml:"network_protocol"`

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

	// Suppresses any HTTP-related logs such as TLS handshake errors.
	SuppressHTTPLogs bool `yaml:"suppress_http_logs"`

	// Disables HTTP keep-alives.
	DisableHTTPKeepAlives bool `yaml:"disable_http_keepalives"`
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


	var address, networkProtocol, urlAddress string
	switch config.NetworkProtocol {
	case "", "tcp":
		if config.Port == 0 {
			address = config.Address
		} else {
			address = fmt.Sprintf("%s:%d", address, config.Port)
		}
		networkProtocol = "tcp"
		urlAddress = address
		if address != "" && address[0] == ':' {
			urlAddress = "[::]" + urlAddress
		}
	case "unix":
		address = config.Address
		networkProtocol = "unix"
		urlAddress = "unix:" + address + ":"
		if config.Port != 0 {
			log.Fatal("The port option is invalid with UNIX sockets.")
		}
	default:
		log.Fatalf("Unrecognised network protocol: %q", config.NetworkProtocol)
	}

	if config.TLS.Enable {
		log.Printf("Starting server on https://%s/", urlAddress)
	} else {
		log.Printf("Starting server on http://%s/", urlAddress)
	}

	// Remove any socket file that already exists
	if networkProtocol == "unix" {
		os.Remove(address)
	}

	// Bind to the address
	var ln net.Listener
	ln, err = net.Listen(networkProtocol, address)
	if err != nil {
		log.Fatal(err)
	}

	// Change permissions on the UNIX socket
	if networkProtocol == "unix" {
		if err := os.Chmod(address, 0777); err != nil {
			log.Fatal(err)
		}
	}

	// Switch to the logfile
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

	// Suppress HTTP logs.
	server := &http.Server{Addr: address, Handler: router}
	if config.SuppressHTTPLogs {
		server.ErrorLog = log.New(ioutil.Discard, "", 0)
	}

	// My laptop doesn't work nicely with Keep-Alive.
	if config.DisableHTTPKeepAlives {
		server.SetKeepAlivesEnabled(false)
	}

	// Serve the webpage
	if config.TLS.Enable {
		err = server.ServeTLS(ln, config.TLS.CertFile, config.TLS.KeyFile)
	} else {
		err = server.Serve(ln)
	}

	log.Fatal(err)
}
