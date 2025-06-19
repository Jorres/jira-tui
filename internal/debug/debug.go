package debug

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func Debug(v ...any) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	dir := filepath.Join(home, ".jira-tui")
	err = os.MkdirAll(dir, 0o755)
	if err != nil {
		return
	}

	logPath := filepath.Join(dir, "debug.log")
	f, err := os.OpenFile(logPath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0o644,
	)

	if err != nil {
		return
	}
	defer f.Close()

	for _, val := range v {
		fmt.Fprint(f, val, " ")
	}

	fmt.Fprintln(f)
}

func Fatal(v ...any) {
	Debug(v...)
	log.Fatal("Exiting in debug.Fatal()...")
}
