// mango - generate man pages from the source of your Go commands
//
// Description:
//
// mango is a small command line utility that allows you to create
// manual pages from the source code of your Go commands.
// It builds manual pages from the comments and flag function calls found in
// your .go files.
//
package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/slyrz/mango/markup"
	"github.com/slyrz/mango/source"
)

var (
	optPath  = ""
	optName  = ""
	optPlain = false
)

func init() {
	// Save manual pages to specified directory.
	flag.StringVar(&optPath, "dir", "", "output directory")

	// Define command name via flag instead of using filename.
	flag.StringVar(&optName, "name", "", "command name")

	// Treat comments as plain text rather than markdown.
	flag.BoolVar(&optPlain, "plain", false, "plain text comments")
}

type Builder struct {
	File      *source.File
	Tokenizer *markup.Tokenizer
	Parser    *markup.Parser
	Renderer  markup.Renderer
	Writer    markup.Writer
}

func NewBuilder() *Builder {
	result := new(Builder)
	result.Tokenizer = markup.NewTokenizer()
	result.Parser = markup.NewParser()
	result.Writer = markup.NewTroffWriter()
	result.Renderer = markup.NewTroffRenderer(result.Writer)
	return result
}

func (b *Builder) Load(path string) error {
	file, err := source.NewFile(path)
	if err != nil {
		return err
	}
	b.File = file

	if len(optName) > 0 {
		b.File.Name = optName
	}
	b.Writer.WriteTitle(b.File.Name)
	b.Writer.WriteDate(b.File.Time)
	b.feedDocumentation()
	b.feedSynopsis()
	b.feedOptions()
	return nil
}

func (b *Builder) feedDocumentation() {
	if optPlain {
		b.Renderer.Section("Name")
		b.Renderer.Text(b.File.Doc)
		return
	}

	tokens, err := b.Tokenizer.TokenizeString(b.File.Doc)
	if err != nil {
		return
	}

	b.Renderer.Section("Name")
	markup.Render(b.Renderer, b.Parser.Parse(tokens))
}

func (b *Builder) feedSynopsis() {
	b.Renderer.Section("Synopsis")
	b.Renderer.Text(b.File.Name)
	if len(b.File.Options) > 0 {
		b.Renderer.TextUnderline("[option...]")
	}
	b.Renderer.TextUnderline("[argument...]")
	b.Renderer.Break()
}

func (b *Builder) feedOptions() {
	if len(b.File.Options) == 0 {
		return
	}

	b.Renderer.Section("Options")
	for _, opt := range b.File.Options {
		textHead := ""
		textType := ""
		textBody := ""

		if len(opt.Short) > 0 {
			textHead = fmt.Sprintf("-%s, -%s", opt.Short, opt.Name)
		} else {
			textHead = fmt.Sprintf("-%s", opt.Name)
		}

		if len(opt.Doc) > 0 {
			textBody = opt.Doc
		} else {
			textBody = opt.Usage
		}

		if opt.Type != "Bool" {
			textType = fmt.Sprintf("<%s>", strings.ToLower(opt.Type))
		}

		tokens := markup.Tokens{
			markup.NewToken(markup.TOKEN_INDENT),
			markup.NewTokenWithText(markup.TOKEN_TEXT, textBody),
			markup.NewToken(markup.TOKEN_EOL),
		}
		if !optPlain {
			// Tokenize body text. We haven't written anything yet, so if Tokenize
			// function fails, the document stays unchanged and we try to parse the
			// next option.
			var err error
			tokens, err = b.Tokenizer.TokenizeString(textBody)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning:")
				continue
			}
		}

		b.Renderer.TextBold(textHead)
		if len(textType) > 0 {
			b.Renderer.Text(textType)
		}
		if len(tokens) > 0 {
			markup.Render(b.Renderer, b.Parser.ParsePart(tokens))
		}
		b.Renderer.Break()
	}
}

func (b *Builder) Save(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	return markup.Save(b.Writer, file)
}

func usage() {
	fmt.Println("Usage:")
	fmt.Println("  mango [option]... [go-file]...\n")
	fmt.Println("Options:")

	flag.CommandLine.VisitAll(func(fl *flag.Flag) {
		name := fl.Name
		help := fl.Usage
		if _, ok := fl.Value.(interface {
			IsBoolFlag()
		}); !ok {
			name = fmt.Sprintf("%s <value>", name)
		}
		fmt.Printf("  -%-18s\n\t%s\n", name, help)
	})
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		return
	}

	for _, srcPath := range flag.Args() {
		builder := NewBuilder()

		if err := builder.Load(srcPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not open file '%s': %s\n", srcPath, err)
			continue
		}

		dstPath := fmt.Sprintf("%s.1", builder.File.Name)
		if len(optPath) > 0 {
			dstPath = path.Join(optPath, dstPath)
		}

		if err := builder.Save(dstPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not save '%s': %s\n", dstPath, err)
			continue
		}
		fmt.Printf("%s -> %s\n", srcPath, dstPath)
	}
}
