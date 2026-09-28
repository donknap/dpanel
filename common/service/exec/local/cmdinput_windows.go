//go:build windows

package local

import (
	"io"
	"sync"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

type cmdPipeInput struct {
	target io.WriteCloser
	echo   io.Writer
	mu     sync.Mutex
	line   []rune
	bytes  []byte
	escape uint8
	lastCR bool
	closed bool
}

func (self *cmdPipeInput) Write(input []byte) (int, error) {
	self.mu.Lock()
	defer self.mu.Unlock()
	if self.closed {
		return 0, io.ErrClosedPipe
	}

	self.bytes = append(self.bytes, input...)
	for len(self.bytes) > 0 && utf8.FullRune(self.bytes) {
		character, size := utf8.DecodeRune(self.bytes)
		self.bytes = self.bytes[size:]
		if self.escape != 0 {
			if self.escape == 1 && (character == '[' || character == 'O') {
				self.escape = 2
			} else if self.escape == 1 || character >= '@' && character <= '~' {
				self.escape = 0
			}
			continue
		}
		if character == '\x1b' {
			self.escape = 1
			continue
		}
		if character == '\n' && self.lastCR {
			self.lastCR = false
			continue
		}
		self.lastCR = character == '\r'

		switch character {
		case '\r', '\n':
			if _, err := self.echo.Write([]byte("\r\n")); err != nil {
				return 0, err
			}
			command := append([]byte(string(self.line)), '\r', '\n')
			self.line = self.line[:0]
			if _, err := self.target.Write(command); err != nil {
				return 0, err
			}
		case '\b', '\x7f':
			if len(self.line) == 0 {
				continue
			}
			last := self.line[len(self.line)-1]
			self.line = self.line[:len(self.line)-1]
			for width := runewidth.RuneWidth(last); width > 0; width-- {
				if _, err := self.echo.Write([]byte("\b \b")); err != nil {
					return 0, err
				}
			}
		default:
			if character < ' ' && character != '\t' {
				continue
			}
			self.line = append(self.line, character)
			if _, err := self.echo.Write([]byte(string(character))); err != nil {
				return 0, err
			}
		}
	}
	return len(input), nil
}

func (self *cmdPipeInput) Close() error {
	self.mu.Lock()
	defer self.mu.Unlock()
	if self.closed {
		return nil
	}
	self.closed = true
	return self.target.Close()
}
