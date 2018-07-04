package formatter

// Mostly copied from https://github.com/docker/cli/blob/7138d6e3011deb4b7216130f64679612b49bdd75/cli/command/formatter/formatter.go

import (
	"bytes"
	"io"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/docker/cli/templates"
	"github.com/pkg/errors"
)

// Format keys used to specify certain kinds of output formats
const (
	TableFormatKey  = "table"
	RawFormatKey    = "raw"
	PrettyFormatKey = "pretty"

	defaultQuietFormat = "{{.ID}}"
)

// Format is the format string rendered using the Context
type Format string

// IsTable returns true if the format is a table-type format
func (f Format) IsTable() bool {
	return strings.HasPrefix(string(f), TableFormatKey)
}

// Contains returns true if the format contains the substring
func (f Format) Contains(sub string) bool {
	return strings.Contains(string(f), sub)
}

// Context contains information required by the formatter to print the output as desired.
type Context struct {
	// Output is the output stream to which the formatted string is written.
	Output io.Writer
	// Format is used to choose raw, table or custom format for the output.
	Format Format
	// Trunc when set to true will truncate the output of certain fields such as Container ID.
	Trunc bool

	// internal element
	finalFormat string
	header      interface{}
	buffer      *bytes.Buffer
}

func (c *Context) preFormat() {
	c.finalFormat = string(c.Format)

	// TODO: handle this in the Format type
	if c.Format.IsTable() {
		c.finalFormat = c.finalFormat[len(TableFormatKey):]
	}

	c.finalFormat = strings.Trim(c.finalFormat, " ")
	r := strings.NewReplacer(`\t`, "\t", `\n`, "\n")
	c.finalFormat = r.Replace(c.finalFormat)
}

func (c *Context) parseFormat() (*template.Template, error) {
	tmpl, err := templates.Parse(c.finalFormat)
	if err != nil {
		return tmpl, errors.Errorf("Template parsing error: %v\n", err)
	}
	return tmpl, err
}

func (c *Context) postFormat(tmpl *template.Template, subContext SubContext) {
	if c.Format.IsTable() {
		t := tabwriter.NewWriter(c.Output, 20, 1, 3, ' ', 0)
		buffer := bytes.NewBufferString("")
		tmpl.Funcs(templates.HeaderFunctions).Execute(buffer, subContext.FullHeader())
		buffer.WriteTo(t)
		t.Write([]byte("\n"))
		c.buffer.WriteTo(t)
		t.Flush()
	} else {
		c.buffer.WriteTo(c.Output)
	}
}

func (c *Context) contextFormat(tmpl *template.Template, subContext SubContext) error {
	if err := tmpl.Execute(c.buffer, subContext); err != nil {
		return errors.Errorf("Template parsing error: %v\n", err)
	}
	if c.Format.IsTable() && c.header != nil {
		c.header = subContext.FullHeader()
	}
	c.buffer.WriteString("\n")
	return nil
}

// SubFormat is a function type accepted by Write()
type SubFormat func(func(SubContext) error) error

// Write the template to the buffer using this Context
func (c *Context) Write(sub SubContext, f SubFormat) error {
	c.buffer = bytes.NewBufferString("")
	c.preFormat()

	tmpl, err := c.parseFormat()
	if err != nil {
		return err
	}

	subFormat := func(subContext SubContext) error {
		return c.contextFormat(tmpl, subContext)
	}
	if err := f(subFormat); err != nil {
		return err
	}

	c.postFormat(tmpl, sub)
	return nil
}

type SubContext interface {
	FullHeader() interface{}
}

// HeaderContext provides the SubContext interface for managing headers
type HeaderContext struct {
	Header interface{}
}

// FullHeader returns the header as an interface
func (c *HeaderContext) FullHeader() interface{} {
	return c.Header
}

func stripNamePrefix(ss []string) []string {
	sss := make([]string, len(ss))
	for i, s := range ss {
		sss[i] = s[1:]
	}

	return sss
}
