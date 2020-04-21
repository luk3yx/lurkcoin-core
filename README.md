# lurkcoin

This is the core code of the next major release of
[lurkcoin](https://forum.minetest.net/viewtopic.php?f=9&t=22768). This is
currently WIP and there will probably be bugs.

## Dependencies

 - [Go](https://golang.org) 1.10+, Go 1.14 or later recommended.
 - [bbolt](https://github.com/etcd-io/bbolt)
    - `go get https://github.com/etcd-io/bbolt`
 - [httprouter](https://github.com/julienschmidt/httprouter)
    - `go get https://github.com/julienschmidt/httprouter`
 - [yaml](https://gopkg.in/yaml.v2)
    - `go get https://gopkg.in/yaml.v2`

## Configuration

See config.yaml for a list of configuration options.

## Compilation flags

The following compilation flags are supported:

 - `lurkcoin.disablebbolt`: Disables the bbolt database. If this flag is used,
    bbolt does not need to be installed.
 - `lurkcoin.disableplaintextdb`: Disables the plaintext database.
 - `lurkcoin.disablev2api`: Disables version 2 of the API. This can also be
    done at runtime in config.yaml.

## License

Copyright © 2020 by luk3yx

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
