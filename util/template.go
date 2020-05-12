package util

import (
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/leg100/stok/crdinfo"
)

var backtickReplacer = strings.NewReplacer("<backtick>", "`")

func GenerateTemplate(crd crdinfo.CRDInfo, contents, path string) {
	// make sure parent dir exists first; drop any errors
	os.MkdirAll(filepath.Dir(path), 0755)

	// create the file
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// replace <backtick> with `
	contents = backtickReplacer.Replace(contents)

	// parse and render template
	tmpl := template.New("")
	tmpl = tmpl.Funcs(template.FuncMap{"ToLowerCamel": strcase.ToLowerCamel})
	tmpl = tmpl.Funcs(template.FuncMap{"ToCamel": strcase.ToCamel})
	tmpl = tmpl.Funcs(template.FuncMap{"ToSnake": strcase.ToSnake})
	template.Must(tmpl.Parse(contents)).Execute(f, crd)
}
