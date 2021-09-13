package gqlformatter_test

import (
	"bytes"
	"flag"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"

	"github.com/diegosz/gqlformatter"
)

var update = flag.Bool("u", false, "update golden files")

func TestFormatQuery(t *testing.T) {
	const testSourceDir = "./testdata/baseline"
	const testBaselineDir = "./testdata/formatted"

	executeGoldenTesting(t, &goldenConfig{
		SourceDir: testSourceDir,
		BaselineFileName: func(cfg *goldenConfig, f os.FileInfo) string {
			return path.Join(testBaselineDir, f.Name())
		},
		Run: func(t *testing.T, cfg *goldenConfig, f os.FileInfo) string {
			input := mustReadFile(path.Join(testSourceDir, f.Name()))
			input = strings.ReplaceAll(input, `\n`, "\n")
			result, err := gqlformatter.FormatQuery(input)
			if err != nil {
				t.Log(result)
				t.Fatal(err)
			}
			return result
		},
	})
}

func TestFormatQueryMinified(t *testing.T) {
	const testSourceDir = "./testdata/baseline"
	const testBaselineDir = "./testdata/minified"

	executeGoldenTesting(t, &goldenConfig{
		SourceDir: testSourceDir,
		BaselineFileName: func(cfg *goldenConfig, f os.FileInfo) string {
			return path.Join(testBaselineDir, f.Name())
		},
		Run: func(t *testing.T, cfg *goldenConfig, f os.FileInfo) string {
			input := mustReadFile(path.Join(testSourceDir, f.Name()))
			input = strings.ReplaceAll(input, `\n`, "\n")
			result, err := gqlformatter.FormatQueryMinified(input)
			if err != nil {
				t.Log(result)
				t.Fatal(err)
			}
			return result
		},
	})
}

type goldenConfig struct {
	SourceDir        string
	IsTarget         func(f os.FileInfo) bool
	BaselineFileName func(cfg *goldenConfig, f os.FileInfo) string
	Run              func(t *testing.T, cfg *goldenConfig, f os.FileInfo) string
}

func executeGoldenTesting(t *testing.T, cfg *goldenConfig) {
	t.Helper()

	if cfg.IsTarget == nil {
		cfg.IsTarget = func(f os.FileInfo) bool {
			return !f.IsDir()
		}
	}
	if cfg.BaselineFileName == nil {
		t.Fatal("BaselineFileName function is required")
	}
	if cfg.Run == nil {
		t.Fatal("Run function is required")
	}

	fs, err := ioutil.ReadDir(cfg.SourceDir)
	if err != nil {
		t.Fatal(fs)
	}

	for _, f := range fs {
		if f.IsDir() {
			continue
		}
		f := f

		t.Run(f.Name(), func(t *testing.T) {
			result := cfg.Run(t, cfg, f)

			expectedFilePath := cfg.BaselineFileName(cfg, f)

			if *update {
				err := os.Remove(expectedFilePath)
				if err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
			}

			expected, err := ioutil.ReadFile(expectedFilePath)
			if os.IsNotExist(err) {
				if *update {
					err = os.MkdirAll(path.Dir(expectedFilePath), 0755)
					if err != nil {
						t.Fatal(err)
					}
					err = ioutil.WriteFile(expectedFilePath, []byte(result), 0664)
					if err != nil {
						t.Fatal(err)
					}
					return
				}
			} else if err != nil {
				t.Fatal(err)
			}

			assert.Equalf(t, string(expected), result, "if you want to accept new result, use -u option")
		})
	}
}

func mustReadFile(name string) string {
	src, err := ioutil.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return string(src)
}

func TestFormatQueryWithIndent(t *testing.T) {
	query := "query{products(where:{and:{id:{greater_or_equals:20} id:{lt:28}}}){id name price}}"
	tests := []struct {
		name   string
		input  string
		indent string
		want   string
	}{
		{
			name:   "no spaces",
			input:  query,
			indent: "",
			want: `query {
products(
where: {
and: {
id: { greater_or_equals: 20 }
id: { lt: 28 }
}
}
) {
id
name
price
}
}
`,
		},
		{
			name:   "1 space",
			input:  query,
			indent: " ",
			want: `query {
 products(
  where: {
   and: {
    id: { greater_or_equals: 20 }
    id: { lt: 28 }
   }
  }
 ) {
  id
  name
  price
 }
}
`,
		},
		{
			name:   "4 spaces",
			input:  query,
			indent: "    ",
			want: `query {
    products(
        where: {
            and: {
                id: { greater_or_equals: 20 }
                id: { lt: 28 }
            }
        }
    ) {
        id
        name
        price
    }
}
`,
		},
		{
			name:   "1 tab",
			input:  query,
			indent: "\t",
			want: `query {
	products(
		where: {
			and: {
				id: { greater_or_equals: 20 }
				id: { lt: 28 }
			}
		}
	) {
		id
		name
		price
	}
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := parser.ParseQuery(&ast.Source{
				Name:  tt.name,
				Input: tt.input,
			})
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			f := gqlformatter.NewFormatter(&buf, gqlformatter.WithIndent(tt.indent))
			f.FormatQueryDocument(doc)
			_, err = parser.ParseQuery(&ast.Source{
				Name:  tt.name,
				Input: buf.String(),
			})
			if err != nil {
				t.Fatal(err)
			}
			if !utf8.Valid(buf.Bytes()) {
				t.Error("invalid UTF-8 encoding")
			}
			got := buf.String()
			if got != tt.want {
				t.Errorf("FormatQuery %q with tab %q = %v, want %v", tt.name, tt.indent, got, tt.want)
			}
		})
	}
}
