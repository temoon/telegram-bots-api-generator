package main

import (
	"errors"
	"golang.org/x/net/html"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const TelegramBotsApiUrl = "https://core.telegram.org/bots/api"

const BlockMethods = "methods"
const BlockTypes = "types"

type Types map[string]*Type

func (t Types) GetFilteredKeys() (keys []string) {
	keys = make([]string, 0, len(t))
	for key, value := range t {
		if isInputFileType(key) || len(value.Subtypes) != 0 {
			continue
		}

		keys = append(keys, key)
	}

	sort.Strings(keys)

	return
}

type Type struct {
	Name     string
	Subtypes []string
	Fields   Fields
}

type Methods map[string]*Method

type Method struct {
	Key        string
	ReturnType string
	Fields     Fields
}

type Fields map[string]*Field

type Field struct {
	Key        string
	Type       string
	IsRequired bool
}

func fetch() (doc *html.Node, err error) {
	var res *http.Response
	if res, err = http.Get(TelegramBotsApiUrl); err != nil {
		return
	}
	//goland:noinspection GoUnhandledErrorResult
	defer res.Body.Close()

	if doc, err = html.Parse(res.Body); err != nil {
		return
	}

	return
}

func parse(doc *html.Node) (methods Methods, types Types, err error) {
	// Content
	findContentOpts := FindOpts{
		Criteria: func(node *html.Node) bool {
			if node.Type == html.ElementNode && node.Data == "div" {
				attrs := getNodeAttributes(node)
				return attrs["id"] == "dev_page_content"
			}

			return false
		},
	}

	if doc = findNextNode(doc, &findContentOpts); doc == nil {
		err = errors.New("content not found")
		return
	}

	// Methods and types
	methods = make(Methods)
	types = make(Types)

	findAnchor := FindOpts{
		Criteria: func(node *html.Node) bool {
			if node.Type == html.ElementNode && node.Data == "a" {
				attrs := getNodeAttributes(node)
				return attrs["class"] == "anchor" && !strings.Contains(attrs["name"], "-")
			}

			return false
		},
	}

	var currentBlock, currentName string
	var desc string
	for node := doc.FirstChild; node != nil; node = node.NextSibling {
		if node.Type != html.ElementNode {
			continue
		}

		if node.Data == "h3" || node.Data == "hr" {
			currentBlock = ""
			currentName = ""
		}

		if node.Data == "h4" {
			findAnchor.ResetCounters()
			anchor := findNextNode(node, &findAnchor)
			if anchor == nil {
				currentBlock = ""
				currentName = ""

				continue
			}

			currentName = getNodeText(node)

			if unicode.IsUpper(rune(currentName[0])) {
				currentBlock = BlockTypes

				types[currentName] = &Type{
					Name:     currentName,
					Subtypes: make([]string, 0),
					Fields:   make(Fields),
				}
			} else {
				currentBlock = BlockMethods
				desc = ""

				methods[currentName] = &Method{
					Key:    currentName,
					Fields: make(Fields),
				}
			}
		}

		if len(currentBlock) == 0 || len(currentName) == 0 {
			continue
		}

		if node.Data == "p" && currentBlock == BlockMethods {
			desc += getBlockDescription(node)
			if m, ok := methods[currentName]; ok && len(desc) > 0 {
				m.ReturnType = getMethodReturnType(desc)
			}
		}

		if node.Data == "table" {
			switch currentBlock {
			case BlockMethods:
				if m, ok := methods[currentName]; ok {
					m.Fields = getBlockFields(node, currentBlock)
				}
			case BlockTypes:
				if t, ok := types[currentName]; ok {
					t.Fields = getBlockFields(node, currentBlock)
				}
			}
		}

		if currentBlock == BlockTypes && node.Data == "ul" {
			if t, ok := types[currentName]; ok {
				t.Subtypes = getBlockSubtypes(node)
			}
		}
	}

	return
}

func getBlockDescription(node *html.Node) string {
	return getNodeText(node)
}

func getBlockFields(node *html.Node, currentBlock string) (fields Fields) {
	fields = make(Fields)

	findBodyOpts := FindOpts{
		Criteria: func(node *html.Node) bool {
			return node.Type == html.ElementNode && node.Data == "tbody"
		},
	}

	tableBody := findNextNode(node, &findBodyOpts)
	if tableBody == nil {
		return
	}

	findRowsOpts := FindOpts{
		Criteria: func(node *html.Node) bool {
			return node.Type == html.ElementNode && node.Data == "tr"
		},
	}

	tableRows := findAllNodes(tableBody, &findRowsOpts)
	if len(tableRows) == 0 {
		return
	}

	findColsOpts := FindOpts{
		Criteria: func(node *html.Node) bool {
			return node.Type == html.ElementNode && node.Data == "td"
		},
	}

	findSendingFiles := FindOpts{
		Criteria: func(node *html.Node) bool {
			if node.Type == html.ElementNode && node.Data == "a" {
				attrs := getNodeAttributes(node)
				return attrs["href"] == "#sending-files"
			}

			return false
		},
	}

	for _, row := range tableRows {
		findColsOpts.ResetCounters()
		findSendingFiles.ResetCounters()

		tableCols := findAllNodes(row, &findColsOpts)
		if currentBlock == BlockMethods && len(tableCols) == 4 {
			key := getNodeText(tableCols[0])

			var fieldType string
			if findNextNode(tableCols[3], &findSendingFiles) != nil {
				fieldType = InputFileType
			} else {
				fieldType = correctType(getNodeText(tableCols[1]))
			}

			fields[key] = &Field{
				Key:        key,
				Type:       fieldType,
				IsRequired: getNodeText(tableCols[2]) == "Yes",
			}
		} else if currentBlock == BlockTypes && len(tableCols) == 3 {
			key := getNodeText(tableCols[0])
			desc := getNodeText(tableCols[2])

			var fieldType string
			if findNextNode(tableCols[2], &findSendingFiles) != nil {
				fieldType = InputFileType
			} else {
				fieldType = correctType(getNodeText(tableCols[1]))
			}

			fields[key] = &Field{
				Key:        key,
				Type:       fieldType,
				IsRequired: !strings.HasPrefix(desc, "Optional"),
			}
		} else {
			log.Fatalln("Unexpected number of columns at fields table")
		}
	}

	return
}

func getBlockSubtypes(node *html.Node) (types []string) {
	types = make([]string, 0)

	findItemsOpts := FindOpts{
		Criteria: func(node *html.Node) bool {
			return node.Type == html.ElementNode && node.Data == "li"
		},
	}

	items := findAllNodes(node, &findItemsOpts)
	for _, item := range items {
		types = append(types, getNodeText(item))
	}

	return
}

func getMethodReturnType(desc string) (returns string) {
	re := regexp.MustCompile(`(?:Returns|On success,).*?((?:[Aa]rray of )?[A-Z]\w+)(?: that were sent| of the sent messages?)? (?:object|is returned|on success)`)
	match := re.FindAllStringSubmatch(desc, -1)

	return correctType(match[0][1])
}

func correctType(t string) string {
	switch t {
	case InputFileType + " or String":
		t = InputFileType
	case "Integer or String":
		t = ChatIdType
	}

	var tt []string
	if strings.HasPrefix(t, "Array of InputMediaAudio") {
		tt = strings.Split(t, " , ")
		tt = append(tt[:len(tt)-1], strings.Split(tt[len(tt)-1], " and ")...)
		for i := 1; i < len(tt); i++ {
			tt[i] = "Array of " + tt[i]
		}
	} else {
		tt = strings.Split(t, " or ")
	}

	var item string
	for i := 0; i < len(tt); i++ {
		item = strings.ToLower(tt[i])

		if strings.HasPrefix(item, "array of ") {
			tt[i] = "[]" + correctType(tt[i][9:])
			continue
		}

		switch item {
		case "messages":
			tt[i] = "Message"
		case "boolean", "true":
			tt[i] = "bool"
		case "float", "float number":
			tt[i] = "float64"
		case "integer", "int":
			tt[i] = "int64"
		case "string":
			tt[i] = "string"
		}
	}

	return strings.Join(tt, " or ")
}
