// @title xashloger
// @version 26.1.12
// @description ...
// last version 26.1.12
package main

import (
	"flag"
	"fmt"
	"os"
	"xashloger/internal/app"
	"xashloger/internal/infra/config"
)

var (
	appVersion = "26.3.23"
)

func main() {
	dev := flag.Bool("dev", false, "run in development mode (load config from ./configs)")
	flag.Parse()

	cfg, err := config.Load(*dev)
	if err != nil {
		fmt.Printf("Config error: %v\n", err)
		os.Exit(0)
	}

	fmt.Printf("xashloger v.%s", appVersion)

	application := app.New(cfg)
	application.Run()
}
