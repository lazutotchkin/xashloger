// @title xashloger
// @version 26.1.12
// @description ...

package main

import (
	"flag"
	"fmt"
	"os"
	"xashloger/internal/app"
	"xashloger/internal/config"
)

var (
	appVersion = "26.1.12"
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
