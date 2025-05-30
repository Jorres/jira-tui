package debug

import (
	"fmt"
	"log"
	"os"
)

func Debug(v ...any) {
	f, _ := os.OpenFile("/home/jorres/hobbies/jira-cli/debug.log", os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	for _, val := range v {
		fmt.Fprintln(f, val)
	}
	f.Close()
}

func Fatal(v ...any) {
	Debug(v...)
	log.Fatal("Exiting in debug.Fatal()...")
}
