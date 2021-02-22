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

package main

import (
	"fmt"
	"github.com/luk3yx/lurkcoin-core/lurkcoin"
	"github.com/luk3yx/lurkcoin-core/lurkcoin/api"
	"log"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("This command takes exactly two arguments.")
		fmt.Println("Usage: ./restore-backup CONFIG BACKUP-FILE")
		os.Exit(1)
	}

	config, err := api.LoadConfig(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	lurkcoin.SeedPRNG()
	lurkcoin.PrintASCIIArt()

	db, err := api.OpenDatabase(config)
	if err != nil {
		log.Fatal(err)
	}

	backupFile := os.Args[2]
	log.Printf(
		"Restoring backup %#v into %#v...\n",
		backupFile,
		config.Database.Location,
	)
	file, err := os.Open(backupFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	err = lurkcoin.RestoreDatabase(db, file)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Database backup restored!")
}
