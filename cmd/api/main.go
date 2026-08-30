// Entry point for the HTTP API server.
package main

import "jungo/internal/app"

func main() {
	app.NewAPIFx().Run()
}
