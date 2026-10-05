package main

import (
	"flag"
	"fmt"
	"os"
)

var checks = map[string]func() error{
	"register-device":          checkRegisterDevice,
	"open-verification-case":   checkOpenVerificationCase,
	"record-measurement":       checkRecordMeasurement,
	"reopen-verification-case": checkReopenVerificationCase,
	"seal-verification":        checkSealVerification,
}

func main() {
	workflow := flag.String("workflow", "", "workflow id to verify")
	flag.Parse()

	run, ok := checks[*workflow]
	if !ok {
		fmt.Fprintf(flag.CommandLine.Output(), "unknown workflow %q\n", *workflow)
		flag.Usage()
		os.Exit(2)
	}
	if err := run(); err != nil {
		fmt.Printf("workflow %s: failed: %v\n", *workflow, err)
		os.Exit(1)
	}
	fmt.Printf("workflow %s: ok\n", *workflow)
}
