package gqlformatter

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

func FormatQuery(input string) (string, error) {
	return formatQuery(input, false)
}

func FormatQueryMinified(input string) (string, error) {
	return formatQuery(input, true)
}

func formatQuery(input string, minified bool) (string, error) {
	if input == "" {
		return "", nil
	}
	input = strings.ReplaceAll(input, `\n`, "\n")
	doc, err := parser.ParseQuery(&ast.Source{
		Name:  "",
		Input: input,
	})
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	var f Formatter
	if minified {
		f = NewFormatter(&buf, WithMinification())
	} else {
		f = NewFormatter(&buf)
	}
	f.FormatQueryDocument(doc)
	_, err = parser.ParseQuery(&ast.Source{
		Name:  "",
		Input: buf.String(),
	})
	if err != nil {
		return "", err
	}
	if !utf8.Valid(buf.Bytes()) {
		return "", errors.New("invalid UTF-8 encoding")
	}
	return buf.String(), nil
}

type Formatter interface {
	FormatQueryDocument(doc *ast.QueryDocument)
}

func NewFormatter(
	w io.Writer,
	opts ...Option) Formatter {
	f := &formatter{ // set defaults
		writer:              w,
		indentUnit:          "  ",
		colonUnit:           ": ",
		optArgumentSplitted: true,
		optWhereLogicalOps:  true,
		optCompressed:       false,
	}
	for _, opt := range opts { // apply the list of options
		opt.apply(f)
	}
	return f
}

type formatter struct {
	writer io.Writer

	indent int

	padNext  bool
	lineHead bool

	indentUnit          string
	colonUnit           string
	optArgumentSplitted bool
	optWhereLogicalOps  bool
	optCompressed       bool
}

func (f *formatter) writeString(s string) {
	_, _ = f.writer.Write([]byte(s))
}

func (f *formatter) writeIndent() *formatter {
	if f.lineHead && !f.optCompressed {
		f.writeString(strings.Repeat(f.indentUnit, f.indent))
	}
	f.lineHead = false
	f.padNext = false

	return f
}

func (f *formatter) WriteNewline() *formatter {
	if !f.optCompressed {
		f.writeString("\n")
	}
	f.lineHead = true
	f.padNext = false

	return f
}

func (f *formatter) WriteWord(word string) *formatter {
	if f.lineHead {
		f.writeIndent()
	}
	if f.padNext {
		f.writeString(" ")
	}
	f.writeString(strings.TrimSpace(word))
	f.padNext = true

	return f
}

func (f *formatter) WriteString(s string) *formatter {
	if f.lineHead {
		f.writeIndent()
	}
	if f.padNext && !f.optCompressed {
		f.writeString(" ")
	}
	f.writeString(s)
	f.padNext = false

	return f
}

func (f *formatter) WriteDescription(s string) *formatter {
	if s == "" {
		return f
	}

	f.WriteString(`"""`).WriteNewline()

	ss := strings.Split(s, "\n")
	for _, s := range ss {
		f.WriteString(s).WriteNewline()
	}

	f.WriteString(`"""`).WriteNewline()

	return f
}

func (f *formatter) IncrementIndent() {
	f.indent++
}

func (f *formatter) DecrementIndent() {
	f.indent--
}

func (f *formatter) NoPadding() *formatter {
	f.padNext = false

	return f
}

func (f *formatter) NeedPadding() *formatter {
	f.padNext = true

	return f
}

func (f *formatter) FormatQueryDocument(doc *ast.QueryDocument) {
	if doc == nil {
		return
	}

	f.FormatOperationList(doc.Operations)
	f.FormatFragmentDefinitionList(doc.Fragments)
}

func (f *formatter) FormatOperationList(lists ast.OperationList) {
	for _, def := range lists {
		f.FormatOperationDefinition(def)
	}
}

func (f *formatter) FormatOperationDefinition(def *ast.OperationDefinition) {
	f.WriteWord(string(def.Operation))
	if def.Name != "" {
		f.WriteWord(def.Name)
	}
	f.FormatVariableDefinitionList(def.VariableDefinitions)
	f.FormatDirectiveList(def.Directives)

	if len(def.SelectionSet) != 0 {
		f.FormatSelectionSet(def.SelectionSet)
		f.WriteNewline()
	}
}

func (f *formatter) FormatDirectiveList(lists ast.DirectiveList) {
	if len(lists) == 0 {
		return
	}

	for _, dir := range lists {
		f.FormatDirective(dir)
	}
}

func (f *formatter) FormatDirective(dir *ast.Directive) {
	f.WriteString("@").WriteWord(dir.Name)
	f.FormatArgumentList(dir.Arguments)
}

func (f *formatter) FormatArgumentList(lists ast.ArgumentList) {
	if len(lists) == 0 {
		return
	}
	doSplitted := false
	if f.optArgumentSplitted {
		if len(lists) > 1 {
			doSplitted = true
		} else {
			a := lists[0]
			if a.Name == "where" && len(a.Value.Children) == 1 {
				c := a.Value.Children[0]
				switch c.Name {
				case "and", "or":
					if len(c.Value.Children) > 1 {
						doSplitted = true
					}
				case "not":
					doSplitted = doWhereSplitted(c)
				}
			}
		}
	}
	f.NoPadding().WriteString("(")
	if doSplitted {
		f.WriteNewline()
		f.IncrementIndent()
		for _, arg := range lists {
			f.FormatArgument(arg)
			f.WriteNewline()
		}
		f.DecrementIndent()
	} else {
		for idx, arg := range lists {
			f.FormatArgument(arg)

			if idx != len(lists)-1 {
				if f.optCompressed {
					f.NoPadding().WriteString(" ")
				} else {
					f.NoPadding().WriteWord(",")
				}
			}
		}
	}
	f.WriteString(")").NeedPadding()
}

func (f *formatter) FormatArgument(arg *ast.Argument) {
	switch arg.Name {
	case "where":
		f.FormatWhereArgument(arg)
	default:
		f.WriteWord(arg.Name).NoPadding().WriteString(f.colonUnit)
		f.FormatValue(arg.Value)
	}
}

func (f *formatter) FormatWhereArgument(arg *ast.Argument) {
	if f.optWhereLogicalOps && len(arg.Value.Children) == 1 {
		switch arg.Value.Children[0].Name {
		case "and", "or":
			if len(arg.Value.Children[0].Value.Children) > 1 {
				f.FormatWhereLogOpArgument(arg)
				return
			}
		case "not":
			if doWhereSplitted(arg.Value.Children[0]) {
				f.FormatWhereLogOpArgument(arg)
				return
			}
		}
	}
	f.WriteWord(arg.Name).NoPadding().WriteString(f.colonUnit)
	f.FormatValue(arg.Value)
}

func doWhereSplitted(elem *ast.ChildValue) bool {
	if elem == nil {
		return false
	}
	if elem.Value == nil {
		return false
	}
	if len(elem.Value.Children) == 0 {
		return false
	}
	switch elem.Name {
	case "and", "or":
		if len(elem.Value.Children) > 1 {
			return true
		}
	case "not":
		if len(elem.Value.Children) == 1 && elem.Value.Children[0] != nil {
			return doWhereSplitted(elem.Value.Children[0])
		}
	}
	return false
}

func (f *formatter) FormatWhereLogOpArgument(arg *ast.Argument) {
	f.WriteWord(arg.Name).NoPadding()
	if f.optCompressed {
		f.WriteString(":{")
	} else {
		f.WriteString(": {")
	}
	f.NeedPadding()
	f.WriteNewline()
	f.IncrementIndent()
	f.FormatWhereLogOpChildValue(arg.Value.Children[0])
	f.DecrementIndent()
	f.WriteString("}").NeedPadding()
}

func (f *formatter) FormatWhereLogOpChildValue(elem *ast.ChildValue) {
	f.WriteString(elem.Name).NoPadding().WriteString(f.colonUnit)
	v := elem.Value
	if v == nil {
		f.WriteString("<nil>")
		return
	}
	switch v.Kind {
	case ast.Variable:
		f.WriteString("$" + v.Raw)
	case ast.IntValue, ast.FloatValue, ast.EnumValue, ast.BooleanValue, ast.NullValue:
		f.WriteString(v.Raw)
	case ast.StringValue, ast.BlockValue:
		f.WriteString(strconv.Quote(v.Raw))
	case ast.ListValue:
		var val []string
		for _, c := range v.Children {
			val = append(val, f.GetValueString(c.Value))
		}
		if f.optCompressed {
			f.WriteString("[" + strings.Join(val, ",") + "]")
		} else {
			f.WriteString("[ " + strings.Join(val, ", ") + " ]").WriteNewline().NeedPadding()
		}
	case ast.ObjectValue:
		switch len(v.Children) {
		case 0:
			f.WriteString("{}")
			if !f.optCompressed {
				f.WriteNewline().NeedPadding()
			}
		case 1:
			c := v.Children[0]
			if c.Name == "and" || c.Name == "or" || c.Name == "not" {
				f.WriteString("{").WriteNewline().NeedPadding()
				f.IncrementIndent()
				f.FormatWhereLogOpChildValue(c)
				f.DecrementIndent()
				f.WriteString("}").WriteNewline().NeedPadding()
			} else {
				if f.optCompressed {
					f.WriteString("{")
					f.FormatChildValue(c)
					f.WriteString("}").NeedPadding()
				} else {
					f.WriteString("{ ")
					f.FormatChildValue(c)
					f.WriteString(" }").WriteNewline().NeedPadding()
				}
			}
		default:
			f.WriteString("{").WriteNewline().NeedPadding()
			f.IncrementIndent()
			for _, c := range v.Children {
				if c.Name == "and" || c.Name == "or" || c.Name == "not" {
					f.FormatWhereLogOpChildValue(c)
				} else {
					f.FormatChildValue(c)
					f.WriteNewline().NeedPadding()
				}
			}
			f.DecrementIndent()
			f.WriteString("}").WriteNewline().NeedPadding()
		}
	default:
		panic(fmt.Errorf("unknown value kind %d", v.Kind))
	}
}

func (f *formatter) FormatFragmentDefinitionList(lists ast.FragmentDefinitionList) {
	for _, def := range lists {
		f.FormatFragmentDefinition(def)
	}
}

func (f *formatter) FormatFragmentDefinition(def *ast.FragmentDefinition) {
	f.WriteWord("fragment").WriteWord(def.Name)
	f.FormatVariableDefinitionList(def.VariableDefinition)
	f.WriteWord("on").WriteWord(def.TypeCondition)
	f.FormatDirectiveList(def.Directives)

	if len(def.SelectionSet) != 0 {
		f.FormatSelectionSet(def.SelectionSet)
		f.WriteNewline()
	}
}

func (f *formatter) FormatVariableDefinitionList(lists ast.VariableDefinitionList) {
	if len(lists) == 0 {
		return
	}

	f.WriteString("(")
	for idx, def := range lists {
		f.FormatVariableDefinition(def)

		if idx != len(lists)-1 {
			if f.optCompressed {
				f.NoPadding().WriteString(",")
			} else {
				f.NoPadding().WriteWord(",")
			}
		}
	}
	f.NoPadding().WriteString(")").NeedPadding()
}

func (f *formatter) FormatVariableDefinition(def *ast.VariableDefinition) {
	f.WriteString("$").WriteWord(def.Variable).NoPadding().WriteString(":")
	f.FormatType(def.Type)

	if def.DefaultValue != nil {
		f.WriteWord("=")
		f.FormatValue(def.DefaultValue)
	}
}

func (f *formatter) FormatSelectionSet(sets ast.SelectionSet) {
	if len(sets) == 0 {
		return
	}

	f.WriteString("{").WriteNewline()
	f.IncrementIndent()

	for idx, sel := range sets {
		f.FormatSelection(sel)

		if f.optCompressed {
			if idx != len(sets)-1 {
				if f.optCompressed {
					f.NoPadding().WriteString(" ")
				} else {
					f.NoPadding().WriteWord(",")
				}
			}
		} else {
			f.WriteNewline()
		}
	}

	f.DecrementIndent()
	f.WriteString("}")
}

func (f *formatter) FormatSelection(selection ast.Selection) {
	switch v := selection.(type) {
	case *ast.Field:
		f.FormatField(v)

	case *ast.FragmentSpread:
		f.FormatFragmentSpread(v)

	case *ast.InlineFragment:
		f.FormatInlineFragment(v)

	default:
		panic(fmt.Errorf("unknown Selection type: %T", selection))
	}
}

func (f *formatter) FormatField(field *ast.Field) {
	if field.Alias != "" && field.Alias != field.Name {
		f.WriteWord(field.Alias).NoPadding().WriteString(f.colonUnit)
	}
	f.WriteWord(field.Name)

	if len(field.Arguments) != 0 {
		f.NoPadding()
		f.FormatArgumentList(field.Arguments)
		f.NeedPadding()
	}

	f.FormatDirectiveList(field.Directives)

	f.FormatSelectionSet(field.SelectionSet)
}

func (f *formatter) FormatFragmentSpread(spread *ast.FragmentSpread) {
	f.WriteWord("...").WriteWord(spread.Name)

	f.FormatDirectiveList(spread.Directives)
}

func (f *formatter) FormatInlineFragment(inline *ast.InlineFragment) {
	f.WriteWord("...")
	if inline.TypeCondition != "" {
		f.WriteWord("on").WriteWord(inline.TypeCondition)
	}

	f.FormatDirectiveList(inline.Directives)

	f.FormatSelectionSet(inline.SelectionSet)
}

func (f *formatter) FormatType(t *ast.Type) {
	if f.optCompressed {
		f.WriteString(t.String())
	} else {
		f.WriteWord(t.String())
	}
}

func (f *formatter) FormatValue(value *ast.Value) {
	f.WriteString(f.GetValueString(value))
}

func (f *formatter) FormatChildValue(elem *ast.ChildValue) {
	f.WriteString(elem.Name + f.colonUnit + f.GetValueString(elem.Value))
}

func (f *formatter) GetValueString(value *ast.Value) string {
	if value == nil {
		return "<nil>"
	}
	switch value.Kind {
	case ast.Variable:
		return "$" + value.Raw
	case ast.IntValue, ast.FloatValue, ast.EnumValue, ast.BooleanValue, ast.NullValue:
		return value.Raw
	case ast.StringValue, ast.BlockValue:
		return strconv.Quote(value.Raw)
	case ast.ListValue:
		var val []string
		for _, elem := range value.Children {
			val = append(val, f.GetValueString(elem.Value))
		}
		if f.optCompressed {
			return "[" + strings.Join(val, ",") + "]"
		}
		return "[ " + strings.Join(val, ", ") + " ]"
	case ast.ObjectValue:
		var val []string
		for _, elem := range value.Children {
			val = append(val, elem.Name+f.colonUnit+f.GetValueString(elem.Value))
		}
		if f.optCompressed {
			return "{" + strings.Join(val, " ") + "}"
		}
		return "{ " + strings.Join(val, ", ") + " }"
	default:
		panic(fmt.Errorf("unknown value kind %d", value.Kind))
	}
}
