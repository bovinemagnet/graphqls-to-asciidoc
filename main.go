package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

const (
	// AsciiDoc tags
	CROSS_REF = "[[%s]]\n"
	L3_TAG    = "=== %s\n\n"
	TABLE_SE  = "|===\n"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: ./program schema.graphql")
	}

	b, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	source := &ast.Source{
		Name:  "GraphQL schema",
		Input: string(b),
	}

	doc, gqlErr := parser.ParseSchema(source)
	if gqlErr != nil {
		log.Fatal(gqlErr)
	}

	definitionsMap := make(map[string]*ast.Definition)
	for _, def := range doc.Definitions {
		definitionsMap[def.Name] = def
	}

	// Sort definitions by name
	sortedDefs := make([]*ast.Definition, len(doc.Definitions))
	copy(sortedDefs, doc.Definitions)
	sort.Slice(sortedDefs, func(i, j int) bool {
		return sortedDefs[i].Name < sortedDefs[j].Name
	})

	// Print heading and current time
	fmt.Println("= GraphQL Documentation")
	fmt.Println(":toc: left")
	fmt.Printf(":revdate: %s\n", time.Now().Format(time.RFC1123))
	fmt.Printf(":commandline: %s\n\n", strings.Join(os.Args, " ")) // Add command line as header attribute

	fmt.Println("\n\n")

	fmt.Println("[IMPORTANT]")
	fmt.Println("====")
	fmt.Println("This is automatically generated from the schema file. +")
	fmt.Println("Do not edit this file directly. +")
	fmt.Println("Last generated _{revdate}_")
	fmt.Println("====")
	// Add a blank line.
	fmt.Println()

	printQueries(sortedDefs, definitionsMap)
	fmt.Println()

	printTypes(sortedDefs, definitionsMap)
	fmt.Println()

	printEnums(sortedDefs, definitionsMap)
	fmt.Println()

}

/**
 * Print the type details
 */
func printTypes(sortedDefs []*ast.Definition, definitionsMap map[string]*ast.Definition) {
	fmt.Println("== Types\n")
	for _, t := range sortedDefs {
		if (t.Kind == ast.Object && t.Name != "Query") || t.Kind == ast.Interface {
			fmt.Printf(CROSS_REF, camelToSnake(t.Name)) // Add anchor
			fmt.Printf(L3_TAG, t.Name)

			if t.Description != "" {
				printAsciiDocTags(t.Description)
				fmt.Println()
			}

			printObjectFields(t, definitionsMap)

			fmt.Println()
		}
	}
}

/**
 * Print the enumeration details
 */
func printEnums(sortedDefs []*ast.Definition, definitionsMap map[string]*ast.Definition) {
	fmt.Println("== Enumerations\n")
	for _, t := range sortedDefs {
		if t.Kind == ast.Enum {
			fmt.Printf(CROSS_REF, camelToSnake(t.Name)) // Add anchor
			fmt.Printf("=== %s\n\n", t.Name)

			if t.Description != "" {
				printAsciiDocTags(t.Description)
				fmt.Println()
			}

			printEnumValues(t)

			fmt.Println("\n")
		}
	}
}

/**
 * Print the query details
 */
func processQueryOld(sortedDefs []*ast.Definition, definitionsMap map[string]*ast.Definition) {
	fmt.Println("== Query")
	for _, t := range sortedDefs {
		if t.Kind == ast.Object && t.Name == "Query" {
			fmt.Printf(CROSS_REF, t.Name) // Add anchor
			//fmt.Printf("=== %s\n\n", t.Name)

			if t.Description != "" {
				printAsciiDocTags(t.Description)
			}

			printQuery(t, definitionsMap)

			fmt.Println()
		}
	}
}

/**
 * Print the query details
 */
func printQueries(sortedDefs []*ast.Definition, definitionsMap map[string]*ast.Definition) {
	fmt.Println("== Query")
	fmt.Println()

	for _, t := range sortedDefs {
		if t.Kind == ast.Object && t.Name == "Query" {
			fmt.Printf(CROSS_REF, camelToSnake(t.Name)) // Add anchor
			fmt.Println()

			if t.Description != "" {
				printAsciiDocTags(t.Description)
			}
			fmt.Println()

			printQueryDetails(t, definitionsMap)

			fmt.Println()
		}
	}
}

/**
 * Print the objects to a table.
 */
func printObjectFields(t *ast.Definition, definitionsMap map[string]*ast.Definition) {
	if len(t.Fields) > 0 {
		if t.Name != "Query" {
			fmt.Printf(".type_%s\n", t.Name) // Add header to table
		}
		fmt.Println("[cols=\"2a,4a,6a\", options=\"header\"]")
		fmt.Println(TABLE_SE)
		if t.Name == "Query" {
			fmt.Println("| Return | Function | Description")
		} else {
			fmt.Println("| Type | Field | Description")
		}

		for _, f := range t.Fields {
			typeName := f.Type.String()

			typeName = processTypeName(typeName, definitionsMap)

			if t.Name == "Query" {
				fmt.Printf("| %s | %s | %s\n", typeName, f.Name, f.Description)
			} else {
				fmt.Printf("| %s | %s | %s\n", typeName, f.Name, f.Description)
			}
		}

		fmt.Println(TABLE_SE)
	}
}

/**
 * Print enum values in a table
 */
func printEnumValues(t *ast.Definition) {
	if len(t.EnumValues) > 0 {
		fmt.Printf(".enum_%s\n", camelToSnake(t.Name))
		fmt.Println("[cols=\"2*a\", options=\"header\"]")
		fmt.Println(TABLE_SE)
		fmt.Println("| Value | Description")

		for _, v := range t.EnumValues {
			fmt.Printf("| %s | %s\n", v.Name, v.Description)
		}
		fmt.Println(TABLE_SE)
	}
}

/**
 * print asciidoc tags.
 */
func printAsciiDocTags(description string) {
	lines := strings.Split(description, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# tag::") || strings.HasPrefix(trimmed, "# end::") {
			fmt.Println("//" + trimmed[1:]) // remove the '#' as AsciiDoc comments start with '//' not '#'
		} else if !strings.HasPrefix(trimmed, "# ") {
			// If it is not a comment, then print the line.
			fmt.Println(trimmed)
		}
	}
}

/**
 * Print query as a table
 */
func printQuery(t *ast.Definition, definitionsMap map[string]*ast.Definition) {
	if len(t.Fields) > 0 {
		fmt.Println("[cols=\"2a,4a,6a,4a\", options=\"header\"]")
		fmt.Println(TABLE_SE)
		if t.Name == "Query" {
			fmt.Println("| Return | Function | Description | params")
		} else {
			fmt.Println("| Type | Field | Description | params")
		}

		for _, f := range t.Fields {
			typeName := f.Type.String()

			typeName = processTypeName(typeName, definitionsMap)

			if t.Name == "Query" {
				fmt.Println("| ", typeName)
				fmt.Println("| ", f.Name)
				fmt.Println("| ")
				fmt.Println(f.Description)
				fmt.Println("|", getArgsString(f.Arguments))
				fmt.Println(" ")
				fmt.Println(" ")
			} else {
				fmt.Printf("| %s | %s | %s | %s \n", typeName, f.Name, f.Description, getArgsString(f.Arguments))
			}
		}
		fmt.Printf("%s", TABLE_SE)
	}
}

/**
 * Get the arguments for reconstructing the method signature
 */
func getArgsString(args ast.ArgumentDefinitionList) string {
	var argsStrings []string

	for _, arg := range args {
		argString := fmt.Sprintf("* `%s : %s", arg.Name, arg.Type.String())
		if arg.DefaultValue != nil {
			argString += fmt.Sprintf(" {%s}`", arg.DefaultValue.String())
		} else {
			argString += "`"
		}

		argsStrings = append(argsStrings, argString)
	}
	return strings.Join(argsStrings, " + \n")
}

/**
 * Get the type arguments.
 */
func getArgsMethodTypeString(args ast.ArgumentDefinitionList) string {
	var argsStrings []string

	for _, arg := range args {
		argString := fmt.Sprintf("%s: %s", arg.Name, arg.Type.String())
		if arg.DefaultValue != nil {
			argString += fmt.Sprintf(" = %s", arg.DefaultValue.String())
		}
		argsStrings = append(argsStrings, argString)
	}

	return strings.Join(argsStrings, ", ")
}

/**
 * Print query details
 */
func printQueryDetails(t *ast.Definition, definitionsMap map[string]*ast.Definition) {
	if len(t.Fields) > 0 {
		for _, f := range t.Fields {
			fmt.Println()
			fmt.Printf("[[query_%s]]\n", f.Name)
			fmt.Println("===", f.Name)
			fmt.Println()
			fmt.Printf(".query_%s\n", f.Name)
			fmt.Println("[source, graphql]")
			fmt.Println("----")
			fmt.Printf("%s(%s): %s\n", f.Name, getArgsMethodTypeString(f.Arguments), f.Type.String())
			fmt.Println("----")
			fmt.Println()
			fmt.Println(f.Description)
			fmt.Println()

			typeName := f.Type.String()

			typeName = processTypeName(typeName, definitionsMap)

			fmt.Printf("*Query Name:* _%s_\n\n", f.Name)

			fmt.Printf("*Return:* %s\n\n", typeName)

			if len(f.Arguments) > 0 {
				fmt.Printf(".Arguments\n")
				fmt.Println(getArgsString(f.Arguments))
			}

		}
	}
}

/**
 * Process the type name, adding the asciidoc links if needed.
 */
func processTypeName(typeName string, definitionsMap map[string]*ast.Definition) string {
	// Assuming typeName may be a list type like "[TypeName]"
	// Trim the square brackets if they exist to extract the actual type name
	trimmedTypeName := strings.Trim(typeName, "[]")

	if definitionsMap[trimmedTypeName] != nil && definitionsMap[trimmedTypeName].Kind == ast.Object {
		// If it's a list type, add back the square brackets
		if strings.HasPrefix(typeName, "[") && strings.HasSuffix(typeName, "]") {
			typeName = fmt.Sprintf("[<<%s,`%s`>>]", trimmedTypeName, trimmedTypeName)
		} else {
			typeName = fmt.Sprintf("<<%s,`%s`>>", trimmedTypeName, trimmedTypeName)
		}
	} else {
		// If it's a type, and it is greater than 0, but is is not a special type, then just wrap it.
		if len(typeName) > 0 {
			typeName = fmt.Sprintf("`%s`", typeName)
		}
	}

	return typeName
}

func camelToSnake(s string) string {
	var result string
	for i, v := range s {
		if unicode.IsUpper(v) {
			if i != 0 {
				result += "_"
			}
			result += strings.ToLower(string(v))
		} else {
			result += string(v)
		}
	}
	return result
}
