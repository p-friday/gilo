package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"

	"golang.org/x/sys/unix"
)

const KILO_VERSION string = "0.0.1"

type editorConfig struct {
	cx, cy     int
	screenrows int
	screencols int
	oldState   unix.Termios
}

var E editorConfig

func CTRL_KEY(k byte) byte {
	return k & 0x1f
}

func enableRawMode() {
	termios, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS)
	if err != nil {
		log.Fatal(err)
	}

	E.oldState = *termios

	termios.Iflag &^= unix.BRKINT | unix.ICRNL | unix.INPCK | unix.ISTRIP | unix.IXON
	termios.Oflag &^= unix.OPOST
	termios.Cflag |= unix.CS8
	termios.Lflag &^= unix.ECHO | unix.ICANON | unix.IEXTEN | unix.ISIG
	termios.Cc[unix.VMIN] = 1
	termios.Cc[unix.VTIME] = 0

	if err := unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TCSETS, termios); err != nil {
		log.Fatal(err)
	}

}

func disableRawMode() {
	if err := unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TCSETS, &E.oldState); err != nil {
		log.Fatal(err)
	}
}

func editorReadKey() (byte, error) {
	reader := bufio.NewReader(os.Stdin)
	buffer := make([]byte, 1)
	for n, err := reader.Read(buffer); n != 1; n, err = reader.Read(buffer) {
		if err != nil {
			return 0, err
		}
	}
	return buffer[0], nil
}

func getWindowSize() (int, int, error) {
	ws, err := unix.IoctlGetWinsize(unix.Stdin, unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 {
		return 0, 0, err
	} else {
		return int(ws.Col), int(ws.Row), nil
	}
}

func editorDrawRows(ab *bytes.Buffer) {
	for y := 0; y <= E.screenrows; y++ {
		if y == E.screenrows/3 {
			welcome := fmt.Sprintf("Gilo editor -- verion %s", KILO_VERSION)
			welcomelen := len(welcome)
			if welcomelen > E.screenrows {
				welcomelen = E.screenrows
			}
			padding := (E.screencols - welcomelen) / 2
			if padding != 0 {
				ab.Write([]byte("~"))
				padding--
			}
			for ; padding > 0; padding-- {
				ab.Write([]byte(" "))
			}

			ab.Write([]byte(welcome)[:welcomelen])
		} else {
			ab.Write([]byte("~"))
		}

		ab.Write([]byte("\x1b[K"))

		if y < E.screencols {
			ab.Write([]byte("\r\n"))
		}
	}
}

func editorRefreshScreen() {
	ab := bytes.Buffer{}

	ab.Write([]byte("\x1b[?25l"))
	ab.Write([]byte("\x1b[H"))

	editorDrawRows(&ab)

	ab.Write([]byte("\x1b[?25h"))
	ab.Write([]byte(fmt.Sprintf("\x1b[%d;%dH", E.cy+1, E.cx+1)))

	os.Stdout.Write(ab.Bytes())
	ab.Reset()
}

func editorMoveCursor(key byte) {
	switch key {
	case 'h':
		E.cx--
	case 'j':
		E.cy++
	case 'k':
		E.cy--
	case 'l':
		E.cx++
	}
}

func editorProcessKeypress() {
	c, err := editorReadKey()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d: %q\r\n", c, c)

	switch c {
	case CTRL_KEY('q'):
		os.Stdout.Write([]byte("\x1b[2J"))
		os.Stdout.Write([]byte("\x1b[H"))
		os.Exit(0)
	case 'h', 'j', 'k', 'l':
		editorMoveCursor(c)
	}
}

func initEditor() {
	E.cx = 0
	E.cy = 0
	cols, rows, err := getWindowSize()
	if err != nil {
		log.Fatal(err)
	}
	E.screencols = cols
	E.screenrows = rows
}

func main() {
	enableRawMode()
	defer disableRawMode()

	initEditor()

	for {
		editorRefreshScreen()
		editorProcessKeypress()
	}
}
