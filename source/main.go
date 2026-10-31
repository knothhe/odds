// Command odds is a macOS menu bar app of small utilities.
package main

import (
	"log"

	"github.com/knothhe/odds/source/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
