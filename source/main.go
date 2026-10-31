// Command odds is a macOS menu bar app of small utilities.
package main

import (
	"flag"
	"log"

	"github.com/knothhe/odds/source/internal/app"
)

func main() {
	preview := flag.Bool("preview", false, "preview with temporary settings and no global shortcuts or login registration")
	cleanup := flag.String("cleanup-install", "", "prepare this exact installed Dev bundle for replacement or uninstall")
	flag.Parse()
	if err := app.Run(*preview, *cleanup); err != nil {
		log.Fatal(err)
	}
}
