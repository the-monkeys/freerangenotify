package main

import (
	"encoding/json"
	"flag"
	documentation "github.com/the-monkeys/freerangenotify/internal/documentation"
	"log"
	"os"
)

func main() {
	check := flag.Bool("check", false, "verify generated references without writing")
	inventory := flag.Bool("inventory", false, "print actual route inventory")
	flag.Parse()
	if *inventory {
		json.NewEncoder(os.Stdout).Encode(documentation.RouteHandlers(true))
		return
	}
	root, e := os.Getwd()
	if e != nil {
		log.Fatal(e)
	}
	if e = documentation.Generate(root, *check); e != nil {
		log.Fatal(e)
	}
}
