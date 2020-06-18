package main

import "fmt"

func access(satAddr string, apiKey string, passphrase string) string {
	return fmt.Sprintf("%s - %s - %s", satAddr, apiKey, passphrase)
}
