package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"strings"
)

// https://www.socketloop.com/tutorials/golang-removes-punctuation-or-defined-delimiter-from-the-user-s-input
const delim = "?!.;,*"

func isDelim(c string) bool {
	if strings.Contains(delim, c) {
		return true
	}
	return false
}
func cleanString(input string) string {

	size := len(input)
	temp := ""
	var prevChar string

	for i := 0; i < size; i++ {
		//fmt.Println(input[i])
		str := string(input[i]) // convert to string for easier operation
		if (str == " " && prevChar != " ") || !isDelim(str) {
			temp += str
			prevChar = str
		} else if prevChar != " " && isDelim(str) {
			temp += " "
		}
	}
	return temp
}

// itob returns an 8-byte big endian representation of v.
func itob(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

// lineCounter counts the number of lines in a reader
func lineCounter(r io.Reader) (int, error) {
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
}

// linesInFile returns the number of lines in a file
func linesInFile(fileName string) (int, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return -1, err
	}
	lines, _ := lineCounter(file)
	file.Close()
	return lines, nil
}