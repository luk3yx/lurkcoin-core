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

// +build lurkcoin.disablev2api

package api

import (
	"github.com/julienschmidt/httprouter"
	"log"
	"lurkcoin"
)

func addV2API(_ *httprouter.Router, _ lurkcoin.Database, _ string) {
	log.Print("lurkcoinV2 API enabled at runtime but disabled " +
		"during compilation.")
}
