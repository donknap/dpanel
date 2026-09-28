//go:build windows

package local

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

var getANSIPage = syscall.NewLazyDLL("kernel32.dll").NewProc("GetACP")

func windowsCodePageEncoding(codePage uintptr) (encoding.Encoding, error) {
	switch codePage {
	case 65001:
		return encoding.Nop, nil
	case 54936:
		return simplifiedchinese.GB18030, nil
	case 936:
		return simplifiedchinese.GBK, nil
	case 932:
		return japanese.ShiftJIS, nil
	case 949:
		return korean.EUCKR, nil
	case 950:
		return traditionalchinese.Big5, nil
	case 874:
		return charmap.Windows874, nil
	case 1250:
		return charmap.Windows1250, nil
	case 1251:
		return charmap.Windows1251, nil
	case 1252:
		return charmap.Windows1252, nil
	case 1253:
		return charmap.Windows1253, nil
	case 1254:
		return charmap.Windows1254, nil
	case 1255:
		return charmap.Windows1255, nil
	case 1256:
		return charmap.Windows1256, nil
	case 1257:
		return charmap.Windows1257, nil
	case 1258:
		return charmap.Windows1258, nil
	default:
		return nil, fmt.Errorf("unsupported Windows ANSI code page %d", codePage)
	}
}

func (self *Local) windowsTerminalStreams(output *io.PipeReader, input *io.PipeWriter) (io.Reader, io.WriteCloser) {
	if self.windowsEncoding == nil {
		return output, input
	}
	terminalOutput := transform.NewReader(output, self.windowsEncoding.NewDecoder())
	terminalInput := &codePageWriter{
		writer: transform.NewWriter(input, self.windowsEncoding.NewEncoder()),
		target: input,
	}
	if !strings.EqualFold(filepath.Base(self.cmd.Path), "cmd.exe") {
		return terminalOutput, terminalInput
	}

	combinedReader, combinedWriter := io.Pipe()
	go func() {
		_, err := io.Copy(combinedWriter, terminalOutput)
		_ = combinedWriter.CloseWithError(err)
	}()
	return combinedReader, &cmdPipeInput{target: terminalInput, echo: combinedWriter}
}

type codePageWriter struct {
	writer *transform.Writer
	target io.WriteCloser
}

func (self *codePageWriter) Write(input []byte) (int, error) {
	return self.writer.Write(input)
}

func (self *codePageWriter) Close() error {
	return errors.Join(self.writer.Close(), self.target.Close())
}
