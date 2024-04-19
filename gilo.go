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

type erow struct {
	size  int
	chars []byte
}

type editorConfig struct {
	cx, cy     int
	screenrows int
	screencols int
	numrows    int
	row        erow
	oldState   unix.Termios
}

var E editorConfig

const (
	ARROW_LEFT = iota + 1000
	ARROW_RIGHT
	ARROW_UP
	ARROW_DOWN
	DELETE_KEY
	HOME_KEY
	END_KEY
	PAGE_UP
	PAGE_DOWN
)

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

func editorReadKey() (int, error) {
	reader := bufio.NewReader(os.Stdin)
	buffer := make([]byte, 1)
	for n, err := reader.Read(buffer); n != 1; n, err = reader.Read(buffer) {
		if err != nil {
			return 0, err
		}
	}

	if buffer[0] == '\x1b' {
		// check if we timeout so that we can just return an escape character
		seq := make([]byte, 3)
		_, err := reader.Read(seq)
		if err != nil {
			return '\x1b', err
		}

		if seq[0] == '[' {
			if seq[1] >= '0' && seq[1] <= '9' {
				if seq[2] == '~' {
					switch seq[1] {
					case '1':
						return HOME_KEY, nil
					case '3':
						return DELETE_KEY, nil
					case '4':
						return END_KEY, nil
					case '5':
						return PAGE_UP, nil
					case '6':
						return PAGE_DOWN, nil
					case '7':
						return HOME_KEY, nil
					case '8':
						return END_KEY, nil
					}
				}
			} else {
				switch seq[1] {
				case 'A':
					return ARROW_UP, nil
				case 'B':
					return ARROW_DOWN, nil
				case 'C':
					return ARROW_RIGHT, nil
				case 'D':
					return ARROW_LEFT, nil
				case 'H':
					return HOME_KEY, nil
				case 'F':
					return END_KEY, nil
				}
			}
		} else if seq[0] == 'O' {
			switch seq[1] {
			case 'H':
				return HOME_KEY, nil
			case 'F':
				return END_KEY, nil
			}
		}

		return '\x1b', nil
	} else {
		return int(buffer[0]), nil
	}
}

func getWindowSize() (int, int, error) {
	ws, err := unix.IoctlGetWinsize(unix.Stdin, unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 {
		return 0, 0, err
	} else {
		return int(ws.Col), int(ws.Row), nil
	}
}

func editorOpen(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var line []byte
	var linelen int
	// line := make([]byte, 1024)
	// linelen, err := file.Read(line)
	if scanner.Scan() {
		line = scanner.Bytes()
		linelen = len(line)
	}

	for linelen > 0 && (line[linelen-1] == '\n' || line[linelen-1] == '\r') {
		linelen -= 1
	}

	E.row.size = linelen
	E.row.chars = make([]byte, linelen+1)
	copy(E.row.chars, line[:linelen])
	E.row.chars[linelen] = '\000'
	E.numrows = 1
}

func editorDrawRows(ab *bytes.Buffer) {
	for y := 0; y < E.screenrows; y++ {
		if y >= E.numrows {
			if E.numrows == 0 && y == E.screenrows/3 {
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
		} else {
			length := E.row.size
			if length > E.screencols {
				length = E.screencols
			}
			_, err := ab.Write(E.row.chars[:length])
			if err != nil {
				log.Fatal(err)
			}
			// fmt.Println(n)
		}

		ab.Write([]byte("\x1b[K"))
		if y < E.screenrows-1 {
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

	// os.Stdout.Write(ab.Bytes())
	ab.WriteTo(os.Stdout)
	ab.Reset()
}

func editorMoveCursor(key int) {
	switch key {
	case ARROW_LEFT:
		if E.cx != 0 {
			E.cx--
		}
	case ARROW_DOWN:
		if E.cy != E.screenrows-1 {
			E.cy++
		}
	case ARROW_UP:
		if E.cy != 0 {
			E.cy--
		}
	case ARROW_RIGHT:
		if E.cx != E.screencols-1 {
			E.cx++
		}
	}
}

func editorProcessKeypress() {
	c, err := editorReadKey()
	if err != nil {
		log.Fatal(err)
	}
	// fmt.Printf("%d: %q\r\n", c, c)

	switch c {
	case int(CTRL_KEY('q')):
		os.Stdout.Write([]byte("\x1b[2J"))
		os.Stdout.Write([]byte("\x1b[H"))
		os.Exit(0)
	case PAGE_UP, PAGE_DOWN:
		for times := E.screenrows; times > 0; times-- {
			if c == PAGE_UP {
				editorMoveCursor(ARROW_UP)
			} else {
				editorMoveCursor(ARROW_DOWN)
			}
		}
	case HOME_KEY:
		E.cx = 0
	case END_KEY:
		E.cx = E.screencols - 1
	case ARROW_LEFT, ARROW_DOWN, ARROW_UP, ARROW_RIGHT:
		editorMoveCursor(c)
	}
}

func initEditor() {
	E.cx = 0
	E.cy = 0
	E.numrows = 0

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

	args := os.Args[1:]
	if len(args) > 0 {
		editorOpen(args[0])
	}

	for {
		editorRefreshScreen()
		editorProcessKeypress()
	}
}
