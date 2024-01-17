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
)

const ApiDir = "api"
const RequestsDir = "requests"
const TemplatesDir = "templates"
const TypesHeaderTemplate = "types_header.tmpl"
const TypesTemplate = "types.tmpl"
const RequestFileTemplate = "request.tmpl"
const TypesFile = "types.go"

//type TelegramType struct {
//	Type   *Type
//	Fields []TelegramTypeField
//}

//type TelegramTypeField struct {
//	Field      *Field
//	Key        string
//	Name       string
//	Type       string
//	IsRequired bool
//}

type TypeTemplateData struct {
	Type   *Type
	Fields []*TypeFieldTemplateData
}

func (d *TypeTemplateData) SortFields() {
	sort.Slice(d.Fields, func(i, j int) bool {
		iRequired := "1"
		if d.Fields[i].Field.IsRequired {
			iRequired = "0"
		}

		jRequired := "1"
		if d.Fields[j].Field.IsRequired {
			jRequired = "0"
		}

		return iRequired+d.Fields[i].Field.Key < jRequired+d.Fields[j].Field.Key
	})
}

type TypeFieldTemplateData struct {
	Field *Field
	Name  string
	Type  string
}

type RequestTemplateData struct {
	Imports              []string
	Method               *Method
	Name                 string
	Fields               []RequestFieldTemplateData
	Files                Files
	ResponseType         string
	ResponseTypeVariants []string
}

func (d *RequestTemplateData) SortFields() {
	sort.Slice(d.Fields, func(i, j int) bool {
		iRequired := "1"
		if d.Fields[i].Field.IsRequired {
			iRequired = "0"
		}

		jRequired := "1"
		if d.Fields[j].Field.IsRequired {
			jRequired = "0"
		}

		return iRequired+d.Fields[i].Field.Key < jRequired+d.Fields[j].Field.Key
	})
}

type RequestFieldTemplateData struct {
	Field       *Field
	Name        string
	Type        string
	IsArray     bool
	IsObject    bool
	IsInputFile bool
	IsChatId    bool
	Variants    [][]RequestFieldTemplateData
}

type Files struct {
	Fields   map[string][]FileField
	Arrays   map[string][]FileField
	Subtypes map[string][]FileSubtype
	Variants map[string][]FileVariants
}

type FileSubtype struct {
	Type   string
	Fields []FileField
}

type FileVariants struct {
	Type    string
	IsArray bool
	Fields  []FileField
}

type FileField struct {
	Name       string
	IsRequired bool
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

	for _, key := range types.GetFilteredKeys() {
		item := types[key]

		fields := make([]*TypeFieldTemplateData, 0, len(item.Fields))
		for _, field := range item.Fields {
			fields = append(fields, &TypeFieldTemplateData{
				Field: field,
				Name:  strcase.ToCamel(field.Key),
				Type:  getGoType(types, field.Type, field.IsRequired, ""),
			})
		}

		data := TypeTemplateData{
			Type:   item,
			Fields: fields,
		}
		data.SortFields()

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
	if file, err = os.Create(filepath.Join(ApiDir, RequestsDir, strcase.ToSnake(method.Key)+".go")); err != nil {
		return
	}
	//goland:noinspection GoUnhandledErrorResult
	defer file.Close()

	imports := map[string]bool{
		"io": true,
	}

	data := RequestTemplateData{
		Method:       method,
		Name:         cases.Title(language.English, cases.NoLower).String(method.Key),
		ResponseType: getGoType(types, method.ReturnType, true, "telegram"),

		Files: Files{
			Fields:   make(map[string][]FileField),
			Arrays:   make(map[string][]FileField),
			Subtypes: make(map[string][]FileSubtype),
			Variants: make(map[string][]FileVariants),
		},
	}

	if t, ok := types[method.ReturnType]; ok && len(t.Subtypes) > 0 {
		data.ResponseTypeVariants = make([]string, 0, len(t.Subtypes))
		for _, subtype := range t.Subtypes {
			data.ResponseTypeVariants = append(data.ResponseTypeVariants, getGoType(types, subtype, false, "telegram"))
		}

		imports["errors"] = true
	}

	fields := make([]RequestFieldTemplateData, 0, len(method.Fields))
	for _, field := range method.Fields {
		typeVariants := strings.Split(field.Type, " or ")
		variantSimples := make([]RequestFieldTemplateData, 0, len(typeVariants))
		variantObjects := make([]RequestFieldTemplateData, 0, len(typeVariants))
		if len(typeVariants) > 1 {
			for _, subtype := range typeVariants {
				isArray := isArrayType(subtype)
				isObject := isObjectType(subtype)
				isInputFile := isInputFileType(subtype)
				isChatId := isChatIdType(subtype)

				if subtype == "int64" || subtype == "float64" {
					imports["strconv"] = true
				} else if isObject && !isInputFile && !isChatId || isArray {
					imports["encoding/json"] = true
				}

				requestFieldVariant := RequestFieldTemplateData{
					Type:        getGoType(types, subtype, true, "telegram"),
					IsArray:     isArray,
					IsObject:    isObject,
					IsInputFile: isInputFile,
					IsChatId:    isChatId,
				}

				if isObject || isArray {
					variantObjects = append(variantObjects, requestFieldVariant)
				} else {
					variantSimples = append(variantSimples, requestFieldVariant)
				}
			}

			imports["errors"] = true
		}

		variants := make([][]RequestFieldTemplateData, 0, len(variantSimples)+1)
		for _, variant := range variantSimples {
			variants = append(variants, []RequestFieldTemplateData{variant})
		}
		if len(variantObjects) > 0 {
			variants = append(variants, variantObjects)
		}

		isArray := isArrayType(field.Type)
		isObject := isObjectType(field.Type)
		isInputFile := isInputFileType(field.Type)
		isChatId := isChatIdType(field.Type)

		if field.Type == "int64" || field.Type == "float64" {
			imports["strconv"] = true
		} else if isObject && !isInputFile && !isChatId || isArray {
			imports["encoding/json"] = true
		}

		requestField := RequestFieldTemplateData{
			Field:       field,
			Name:        strcase.ToCamel(field.Key),
			Type:        getGoType(types, field.Type, field.IsRequired, "telegram"),
			IsArray:     isArray,
			IsObject:    isObject,
			IsInputFile: isInputFile,
			IsChatId:    isChatId,
			Variants:    variants,
		}

		fields = append(fields, requestField)

		if t, ok := types[field.Type]; ok && len(t.Subtypes) > 0 {
			subtypes := make([]FileSubtype, 0, len(t.Subtypes))
			for _, subtype := range t.Subtypes {
				if inputFileFields := getInputFileFields(types[subtype].Fields); len(inputFileFields) > 0 {
					subtypes = append(subtypes, FileSubtype{
						Type:   getGoType(types, subtype, true, "telegram"),
						Fields: inputFileFields,
					})
				}
			}

			if len(subtypes) > 0 {
				data.Files.Subtypes[requestField.Name] = subtypes
			}
		} else if typeVariants = strings.Split(field.Type, " or "); len(typeVariants) > 1 {
			vars := make([]FileVariants, 0, len(typeVariants))
			for i, variantType := range typeVariants {
				isArray = isArrayType(variantType)
				if isArray {
					variantType = variantType[2:]
				}

				if inputFileFields := getInputFileFields(types[variantType].Fields); len(inputFileFields) > 0 {
					vars = append(vars, FileVariants{
						Type:    getGoType(types, typeVariants[i], true, "telegram"),
						IsArray: isArray,
						Fields:  inputFileFields,
					})
				}
			}

			if len(vars) > 0 {
				data.Files.Variants[requestField.Name] = vars
			}
		} else if isObject {
			if t, ok = types[field.Type]; ok {
				if inputFileFields := getInputFileFields(t.Fields); len(inputFileFields) > 0 {
					data.Files.Fields[requestField.Name] = inputFileFields
				}
			}
		} else if isArray {
			if t, ok = types[field.Type[2:]]; ok { // len("[]") == 2
				if inputFileFields := getInputFileFields(t.Fields); len(inputFileFields) > 0 {
					data.Files.Arrays[requestField.Name] = inputFileFields
				}
			}
		}
	}

	data.Fields = fields
	data.SortFields()

	data.Imports = make([]string, 0)
	for module := range imports {
		data.Imports = append(data.Imports, module)
	}
	sort.Strings(data.Imports)

	if err = tmpl.ExecuteTemplate(file, RequestFileTemplate, &data); err != nil {
		return
	}

	return
}

func getInputFileFields(fields Fields) (output []FileField) {
	output = make([]FileField, 0)
	for _, field := range fields {
		if !isInputFileType(field.Type) {
			continue
		}

		output = append(output, FileField{
			Name:       strcase.ToCamel(field.Key),
			IsRequired: field.IsRequired,
		})
	}

	return
}

func getGoType(types Types, value string, isRequired bool, pkg string) (t string) {
	hasSubtypes := false
	if t, ok := types[value]; ok && len(t.Subtypes) > 0 {
		hasSubtypes = true
	}

	hasVariants := len(strings.Split(value, " or ")) > 1

	isArray := isArrayType(value) && !hasVariants
	isObject := isObjectType(value) && !hasVariants

	if isObject && !hasSubtypes {
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
				t = "[]" + getGoType(types, value[2:], true, pkg) // len("[]") = 2
			} else {
				t = "interface{}"
			}
		}
	}

	return
}
