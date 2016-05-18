package console

import (
	"fmt"
	"io"
	stdlog "log"
	"os"
	"strconv"

	"github.com/go-playground/log"
)

// FormatFunc is the function that the workers use to create
// a new Formatter per worker allowing reusable go routine safe
// variable to be used within your Formatter function.
type FormatFunc func() Formatter

// Formatter is the function used to format the Redis entry
type Formatter func(e *log.Entry) []byte

const (
	defaultTS = "2006-01-02T15:04:05.000000000Z07:00"
	space     = byte(' ')
	equals    = byte('=')
	newLine   = byte('\n')
	colon     = byte(':')
	base10    = 10
	v         = "%v"
	gopath    = "GOPATH"
)

// Console is an instance of the console logger
type Console struct {
	buffer          uint
	numWorkers      uint
	colors          [9]log.ANSIEscSeq
	writer          io.Writer
	formatFunc      FormatFunc
	timestampFormat string
	gopath          string
	fileDisplay     log.FilenameDisplay
	displayColor    bool
}

// Colors mapping.
var defaultColors = [...]log.ANSIEscSeq{
	log.DebugLevel:  log.Green,
	log.TraceLevel:  log.White,
	log.InfoLevel:   log.Blue,
	log.NoticeLevel: log.LightCyan,
	log.WarnLevel:   log.Yellow,
	log.ErrorLevel:  log.LightRed,
	log.PanicLevel:  log.Red,
	log.AlertLevel:  log.Red + log.Underscore,
	log.FatalLevel:  log.Red + log.Underscore + log.Blink,
}

// New returns a new instance of the console logger
func New() *Console {
	c := &Console{
		buffer:          0,
		numWorkers:      1,
		colors:          defaultColors,
		writer:          os.Stderr,
		timestampFormat: defaultTS,
		displayColor:    true,
		fileDisplay:     log.Lshortfile,
	}

	c.formatFunc = c.defaultFormatFunc

	return c
}

// SetFilenameDisplay tells Console the filename, when present, how to display
func (c *Console) SetFilenameDisplay(fd log.FilenameDisplay) {
	c.fileDisplay = fd
}

// DisplayColor tells Console to output in color or not
// Default is : true
func (c *Console) DisplayColor(color bool) {
	c.displayColor = color
}

// SetTimestampFormat sets Console's timestamp output format
// Default is : "2006-01-02T15:04:05.000000000Z07:00"
func (c *Console) SetTimestampFormat(format string) {
	c.timestampFormat = format
}

// SetWriter sets Console's wriiter
// Default is : os.Stderr
func (c *Console) SetWriter(w io.Writer) {
	c.writer = w
}

// SetBuffersAndWorkers sets the channels buffer size and number of concurrent workers.
// These settings should be thought about together, hence setting both in the same function.
func (c *Console) SetBuffersAndWorkers(size uint, workers uint) {
	c.buffer = size

	if workers == 0 {
		// just in case no log registered yet
		stdlog.Println("Invalid number of workers specified, setting to 1")
		log.Warn("Invalid number of workers specified, setting to 1")

		workers = 1
	}

	c.numWorkers = workers
}

// SetFormatFunc sets FormatFunc each worker will call to get
// a Formatter func
func (c *Console) SetFormatFunc(fn FormatFunc) {
	c.formatFunc = fn
}

// Run starts the logger consuming on the returned channed
func (c *Console) Run() chan<- *log.Entry {

	// pre-setup
	if c.fileDisplay == log.Llongfile {
		// gather $GOPATH for use in stripping off of full name
		// if not found still ok as will be blank
		c.gopath = os.Getenv(gopath)
		if len(c.gopath) != 0 {
			c.gopath += string(os.PathSeparator) + "src" + string(os.PathSeparator)
		}
	}

	// in a big high traffic app, set a higher buffer
	ch := make(chan *log.Entry, c.buffer)

	for i := 0; i <= int(c.numWorkers); i++ {
		go c.handleLog(ch)
	}

	return ch
}

// handleLog consumes and logs any Entry's passed to the channel
func (c *Console) handleLog(entries <-chan *log.Entry) {

	var e *log.Entry
	// var b io.WriterTo
	var b []byte
	formatter := c.formatFunc()

	for e = range entries {

		b = formatter(e)
		c.writer.Write(b)
		// b.WriteTo(c.writer)

		e.Consumed()
	}
}

func (c *Console) defaultFormatFunc() Formatter {

	var b []byte
	var file string
	var lvl string
	var i int

	if c.displayColor {

		var color log.ANSIEscSeq

		return func(e *log.Entry) []byte {
			b = b[0:0]
			color = c.colors[e.Level]

			if e.Line == 0 {

				b = append(b, e.Timestamp.Format(c.timestampFormat)...)
				b = append(b, space)
				b = append(b, color...)

				lvl = e.Level.String()

				for i = 0; i < 6-len(lvl); i++ {
					b = append(b, space)
				}
				b = append(b, lvl...)
				b = append(b, log.Reset...)
				b = append(b, space)
				b = append(b, e.Message...)

			} else {
				file = e.File

				if c.fileDisplay == log.Lshortfile {

					for i = len(file) - 1; i > 0; i-- {
						if file[i] == '/' {
							file = file[i+1:]
							break
						}
					}
				} else {
					file = file[len(c.gopath):]
				}

				b = append(b, e.Timestamp.Format(c.timestampFormat)...)
				b = append(b, space)
				b = append(b, color...)

				lvl = e.Level.String()

				for i = 0; i < 6-len(lvl); i++ {
					b = append(b, space)
				}

				b = append(b, lvl...)
				b = append(b, log.Reset...)
				b = append(b, space)
				b = append(b, file...)
				b = append(b, colon)
				b = strconv.AppendInt(b, int64(e.Line), base10)
				b = append(b, space)
				b = append(b, e.Message...)
			}

			for _, f := range e.Fields {
				b = append(b, space)
				b = append(b, color...)
				b = append(b, f.Key...)
				b = append(b, log.Reset...)
				b = append(b, equals)

				switch f.Value.(type) {
				case string:
					b = append(b, f.Value.(string)...)
				case int:
					b = strconv.AppendInt(b, int64(f.Value.(int)), base10)
				case int8:
					b = strconv.AppendInt(b, int64(f.Value.(int8)), base10)
				case int16:
					b = strconv.AppendInt(b, int64(f.Value.(int16)), base10)
				case int32:
					b = strconv.AppendInt(b, int64(f.Value.(int32)), base10)
				case int64:
					b = strconv.AppendInt(b, f.Value.(int64), base10)
				case uint:
					b = strconv.AppendUint(b, uint64(f.Value.(uint)), base10)
				case uint8:
					b = strconv.AppendUint(b, uint64(f.Value.(uint8)), base10)
				case uint16:
					b = strconv.AppendUint(b, uint64(f.Value.(uint16)), base10)
				case uint32:
					b = strconv.AppendUint(b, uint64(f.Value.(uint32)), base10)
				case uint64:
					b = strconv.AppendUint(b, f.Value.(uint64), base10)
				case bool:
					b = strconv.AppendBool(b, f.Value.(bool))
				default:
					b = append(b, fmt.Sprintf(v, f.Value)...)
				}
			}

			b = append(b, newLine)

			return b
		}
	}

	return func(e *log.Entry) []byte {
		b = b[0:0]

		if e.Line == 0 {

			b = append(b, e.Timestamp.Format(c.timestampFormat)...)
			b = append(b, space)

			lvl = e.Level.String()

			for i = 0; i < 6-len(lvl); i++ {
				b = append(b, space)
			}

			b = append(b, lvl...)
			b = append(b, space)
			b = append(b, e.Message...)

		} else {
			file = e.File

			if c.fileDisplay == log.Lshortfile {

				for i = len(file) - 1; i > 0; i-- {
					if file[i] == '/' {
						file = file[i+1:]
						break
					}
				}
			} else {
				file = file[len(c.gopath):]
			}

			b = append(b, e.Timestamp.Format(c.timestampFormat)...)
			b = append(b, space)

			lvl = e.Level.String()

			for i = 0; i < 6-len(lvl); i++ {
				b = append(b, space)
			}

			b = append(b, lvl...)
			b = append(b, space)
			b = append(b, file...)
			b = append(b, colon)
			b = strconv.AppendInt(b, int64(e.Line), base10)
			b = append(b, space)
			b = append(b, e.Message...)
		}

		for _, f := range e.Fields {
			b = append(b, space)
			b = append(b, f.Key...)
			b = append(b, equals)

			switch f.Value.(type) {
			case string:
				b = append(b, f.Value.(string)...)
			case int:
				b = strconv.AppendInt(b, int64(f.Value.(int)), base10)
			case int8:
				b = strconv.AppendInt(b, int64(f.Value.(int8)), base10)
			case int16:
				b = strconv.AppendInt(b, int64(f.Value.(int16)), base10)
			case int32:
				b = strconv.AppendInt(b, int64(f.Value.(int32)), base10)
			case int64:
				b = strconv.AppendInt(b, f.Value.(int64), base10)
			case uint:
				b = strconv.AppendUint(b, uint64(f.Value.(uint)), base10)
			case uint8:
				b = strconv.AppendUint(b, uint64(f.Value.(uint8)), base10)
			case uint16:
				b = strconv.AppendUint(b, uint64(f.Value.(uint16)), base10)
			case uint32:
				b = strconv.AppendUint(b, uint64(f.Value.(uint32)), base10)
			case uint64:
				b = strconv.AppendUint(b, f.Value.(uint64), base10)
			case bool:
				b = strconv.AppendBool(b, f.Value.(bool))
			default:
				b = append(b, fmt.Sprintf(v, f.Value)...)
			}
		}

		b = append(b, newLine)

		return b
	}
}
