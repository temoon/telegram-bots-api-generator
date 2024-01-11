package main

//go:generate go run .
//go:generate gofmt -w api/types.go
//go:generate gofmt -w api/requests

import (
	"github.com/iancoleman/strcase"
	"golang.org/x/net/html"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"unicode"
)

const ApiDir = "api"
const RequestsDir = "requests"
const TemplatesDir = "templates"
const TypesHeaderTemplate = "types_header.tmpl"
const TypesTemplate = "types.tmpl"
const RequestFileTemplate = "request.tmpl"
const TypesFile = "types.go"

type TelegramType struct {
	Type   *Type
	Name   string
	Fields []TelegramTypeField
}

type TelegramTypeField struct {
	Field      *Field
	Key        string
	Name       string
	Type       string
	IsRequired bool
}

type TelegramRequest struct {
	Imports          []string
	Method           *Method
	Type             string
	Fields           map[string]TelegramRequestField
	HasMediaContent  bool
	ResponseType     string
	ResponseVariants []string
}

type TelegramRequestField struct {
	Name        string
	Type        string
	FieldType   string
	IsRequired  bool
	IsArray     bool
	IsObject    bool
	IsInputFile bool
	Variants    [][]TelegramRequestField
}

func main() {
	var err error

	var doc *html.Node
	if doc, err = fetch(); err != nil {
		log.Fatalln(err)
	}

	var methods Methods
	var types Types
	if methods, types, err = parse(doc); err != nil {
		log.Fatalln(err)
	}

	if err = generateTypes(types); err != nil {
		log.Fatalln(err)
	}

	if err = generateRequests(types, methods); err != nil {
		log.Fatalln(err)
	}
}

func generateTypes(types Types) (err error) {
	var file *os.File
	if file, err = os.Create(filepath.Join(ApiDir, TypesFile)); err != nil {
		return
	}
	//goland:noinspection GoUnhandledErrorResult
	defer file.Close()

	var tmpl *template.Template
	if tmpl, err = template.ParseFiles(filepath.Join(TemplatesDir, TypesHeaderTemplate), filepath.Join(TemplatesDir, TypesTemplate)); err != nil {
		return
	}

	if err = tmpl.ExecuteTemplate(file, TypesHeaderTemplate, nil); err != nil {
		return
	}

	for _, name := range types.GetFilteredNames() {
		item := types[name]

		fields := make([]TelegramTypeField, 0, len(item.Fields))
		for _, field := range item.Fields {
			field := TelegramTypeField{
				Key:        field.Name,
				Name:       strcase.ToCamel(field.Name),
				Type:       generateValueType(types, field.Type, field.IsRequired, ""),
				IsRequired: field.IsRequired,
			}

			fields = append(fields, field)
		}
		sort.Slice(fields, func(i, j int) bool {
			iRequired := "1"
			if fields[i].IsRequired {
				iRequired = "0"
			}

			jRequired := "1"
			if fields[j].IsRequired {
				jRequired = "0"
			}

			return iRequired+fields[i].Key < jRequired+fields[j].Key
		})

		data := TelegramType{
			Name:   name,
			Fields: fields,
		}

		if err = tmpl.ExecuteTemplate(file, TypesTemplate, data); err != nil {
			return
		}
	}

	return
}

func generateRequests(types Types, methods Methods) (err error) {
	requestsDirPath := filepath.Join(ApiDir, RequestsDir)
	if err = os.RemoveAll(requestsDirPath); err != nil {
		return
	}

	if err = os.Mkdir(requestsDirPath, 0o755); err != nil {
		return
	}

	var tmpl *template.Template
	if tmpl, err = template.ParseFiles(filepath.Join(TemplatesDir, RequestFileTemplate)); err != nil {
		return
	}

	for _, item := range methods {
		if err = generateRequestFile(tmpl, types, item); err != nil {
			return
		}
	}

	return
}

func generateRequestFile(tmpl *template.Template, types Types, method *Method) (err error) {
	var file *os.File
	if file, err = os.Create(filepath.Join(ApiDir, RequestsDir, strcase.ToSnake(method.Name)+".go")); err != nil {
		return
	}
	//goland:noinspection GoUnhandledErrorResult
	defer file.Close()

	imports := make(map[string]bool)

	telegramRequest := TelegramRequest{
		Method:          method,
		Type:            cases.Title(language.English, cases.NoLower).String(method.Name),
		Fields:          make(map[string]TelegramRequestField),
		HasMediaContent: method.Fields.HasMediaContent(),
		ResponseType:    generateValueType(types, method.Returns, true, "telegram"),
	}

	if t, ok := types[method.Returns]; ok && len(t.Subtypes) > 0 {
		telegramRequest.ResponseVariants = make([]string, 0, len(t.Subtypes))
		for _, subtype := range t.Subtypes {
			telegramRequest.ResponseVariants = append(telegramRequest.ResponseVariants, generateValueType(types, subtype, false, "telegram"))
		}

		imports["errors"] = true
	}

	for name, field := range method.Fields {
		subtypes := strings.Split(field.Type, " or ")
		variantSimples := make([]TelegramRequestField, 0, len(subtypes))
		variantObjects := make([]TelegramRequestField, 0, len(subtypes))
		if len(subtypes) > 1 {
			for _, subtype := range subtypes {
				isArray := strings.HasPrefix(subtype, "[]")
				isObject := unicode.IsUpper(rune(subtype[0]))
				isInputFile := subtype == IoStreamType

				if subtype == "int64" || subtype == "float64" {
					imports["strconv"] = true
				} else if isInputFile {
					imports["io"] = true
				} else if isObject || isArray {
					imports["encoding/json"] = true
				}

				variant := TelegramRequestField{
					Type:        generateValueType(types, subtype, true, "telegram"),
					IsArray:     isArray,
					IsObject:    isObject,
					IsInputFile: isInputFile,
				}

				if isObject || isArray {
					variantObjects = append(variantObjects, variant)
				} else {
					variantSimples = append(variantSimples, variant)
				}
			}

			imports["errors"] = true
		}

		variants := make([][]TelegramRequestField, 0, len(variantSimples)+1)
		for _, variant := range variantSimples {
			variants = append(variants, []TelegramRequestField{variant})
		}
		if len(variantObjects) > 0 {
			variants = append(variants, variantObjects)
		}

		telegramRequestField := TelegramRequestField{
			Name:        strcase.ToCamel(name),
			Type:        field.Type,
			FieldType:   generateValueType(types, field.Type, field.IsRequired, "telegram"),
			IsRequired:  field.IsRequired,
			IsArray:     field.IsArray(),
			IsObject:    field.IsObject(),
			IsInputFile: field.IsInputFile(),
			Variants:    variants,
		}

		if field.Type == "int64" || field.Type == "float64" {
			imports["strconv"] = true
		} else if field.IsObject() || field.IsArray() {
			imports["encoding/json"] = true
		} else if field.IsInputFile() {
			imports["io"] = true
		}

		telegramRequest.Fields[name] = telegramRequestField
	}

	telegramRequest.Imports = make([]string, 0)
	for str := range imports {
		telegramRequest.Imports = append(telegramRequest.Imports, str)
	}
	sort.Strings(telegramRequest.Imports)

	if err = tmpl.ExecuteTemplate(file, RequestFileTemplate, &telegramRequest); err != nil {
		return
	}

	return
}

func generateValueType(types Types, value string, isRequired bool, pkg string) (t string) {
	isArray := strings.HasPrefix(value, "[]")
	isObject := unicode.IsUpper(rune(value[0]))

	isContainer := false
	if t, ok := types[value]; ok && len(t.Subtypes) > 0 {
		isContainer = true
	}

	if isObject && !isContainer {
		t = value
		if pkg != "" {
			t = pkg + "." + t
		}

		if !isRequired {
			t = "*" + t
		}
	} else {
		switch value {
		case "string", "int64", "float64", "bool":
			t = value
			if !isRequired {
				t = "*" + value
			}
		default:
			if isArray {
				t = "[]" + generateValueType(types, value[2:], true, pkg) // len("[]") = 2
			} else {
				t = "interface{}"
			}
		}
	}

	return
}
