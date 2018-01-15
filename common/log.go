package common

import (
	"log"
	"os"
	"path/filepath"
)

var (
	Log *log.Logger
)

func NewLog(logpath string) {
	println("LogFile: " + logpath)

	dir, _ := filepath.Abs(filepath.Dir(logpath))
	os.Mkdir(dir, 0700)

	file, err := os.Create(logpath)
	if err != nil {
		panic(err)
	}
	Log = log.New(file, "", log.LstdFlags|log.Lshortfile)
}
