package main

import (
	"log"
	"os"
)

/**
 * reportError is the centralized error reporting function.
 *
 * All unexpected errors must be funneled through this function to ensure
 * consistent logging and potential future integration with error tracking
 * systems (like Sentry).
 */
func reportError(err error, context map[string]any) {
	if len(context) > 0 {
		log.Printf("ERROR: %v | Context: %v", err, context)
	} else {
		log.Printf("ERROR: %v", err)
	}
}

/**
 * reportFatalError logs a fatal error through the centralized reporter
 * and exits the program.
 */
func reportFatalError(err error, context map[string]any) {
	reportError(err, context)
	os.Exit(1)
}
