package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"gear/internal/evidencepack"
)

func main() {
	root := flag.String("root", "evidence", "evidence root directory")
	output := flag.String("output", "evidence-pack.tgz", "evidence archive path")
	manifestOutput := flag.String("manifest", "evidence-pack-manifest.json", "manifest JSON path")
	flag.Parse()

	manifest, err := evidencepack.Write(*root, *output, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*manifestOutput, append(data, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %s and %s\n", *output, *manifestOutput)
}
