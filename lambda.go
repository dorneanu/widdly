// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option)
// any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General
// Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program.  If not, see <http://www.gnu.org/licenses/>.

//+build lambda

package main

import (
	"fmt"
	"log"

	"github.com/apex/gateway"
	"github.com/kelseyhightower/envconfig"
	"gitlab.com/opennota/widdly/api"
	"gitlab.com/opennota/widdly/store"
	_ "gitlab.com/opennota/widdly/store/dynamodb"
)

// Required ENV variables
type config struct {
	ENTRYPOINT string `required:"true"`
	WIKIFILE   string `required:"true"`
	URLPATH    string `required:"true"`
	READONLY   bool   `default: false`
}

func main() {
	// Process ENV variables
	var conf config
	err := envconfig.Process("", &conf)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Open store (should be DynamoDB)
	api.Store = store.MustOpen(conf.ENTRYPOINT)

	// Init status information
	// Init status information
	api.ReadOnly = conf.READONLY
	api.SetStatus()

	// Set default URL path
	api.DefaultURLPath = fmt.Sprintf("%s", conf.URLPATH)
	api.InitRoutes()

	// Listen for incoming APIGateway requests
	gateway.ListenAndServe("", api.ServeMux)
}
