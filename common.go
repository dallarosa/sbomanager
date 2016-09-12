package main

import (
	"log"
	"os"
	"runtime"
)

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func getLine() int {
	_, _, line, _ := runtime.Caller(1)
	return line
}

func check(err error, line ...int) {
	if err != nil {
		log.Fatalln(err, line)
		return
	}
}
