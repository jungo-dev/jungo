// Command console executes registered CLI commands.
//
// Usage:
//
//	make console CMD="user:list"
//	make console CMD="health:check"
package main

import "jungo/internal/app"

func main() {
	app.NewConsoleFx().Run()
}
