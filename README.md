# graphqls-to-asciidoc
Simple converter that takes a graphql schema, and produces simple a Asciidoc file.


To run :`./graphqls-to-asciidoc ./schema.graphqls > test.adoc`

This will create a file called `test.adoc` based on the `schema.graphqls` file.

Your schema must be valid as this code is dumb, and relies on the schema parsing of [vektah/gqlparser](https://github.com/vektah/gqlparser).