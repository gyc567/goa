package codegen

import (
	"fmt"
	"strings"
	"text/template"

	"goa.design/goa.v2/design"
)

var (
	simplePublicizeT    *template.Template
	recursivePublicizeT *template.Template
	objectPublicizeT    *template.Template
	arrayPublicizeT     *template.Template
	hashPublicizeT      *template.Template
)

func init() {
	var err error
	fm := template.FuncMap{
		"tabs":                Tabs,
		"goify":               Goify,
		"gotyperef":           GoTypeRef,
		"gotypedef":           GoTypeDef,
		"add":                 Add,
		"publicizer":          Publicizer,
		"recursivePublicizer": RecursivePublicizer,
	}
	if simplePublicizeT, err = template.New("simplePublicize").Funcs(fm).Parse(simplePublicizeTmpl); err != nil {
		panic(err)
	}
	if recursivePublicizeT, err = template.New("recursivePublicize").Funcs(fm).Parse(recursivePublicizeTmpl); err != nil {
		panic(err)
	}
	if objectPublicizeT, err = template.New("objectPublicize").Funcs(fm).Parse(objectPublicizeTmpl); err != nil {
		panic(err)
	}
	if arrayPublicizeT, err = template.New("arrPublicize").Funcs(fm).Parse(arrayPublicizeTmpl); err != nil {
		panic(err)
	}
	if hashPublicizeT, err = template.New("hashPublicize").Funcs(fm).Parse(hashPublicizeTmpl); err != nil {
		panic(err)
	}
}

// RecursivePublicizer produces code that copies fields from the private struct to the
// public struct
func RecursivePublicizer(att *design.AttributeExpr, source, target string, depth int) string {
	var publications []string
	if o := design.AsObject(att.Type); o != nil {
		if ut, ok := att.Type.(design.UserType); ok {
			att = ut.Attribute()
		}
		o.WalkAttributes(func(n string, catt *design.AttributeExpr) error {
			publication := Publicizer(
				catt,
				fmt.Sprintf("%s.%s", source, Goify(n, true)),
				fmt.Sprintf("%s.%s", target, Goify(n, true)),
				design.IsPrimitive(catt.Type) && !att.IsPrimitivePointer(n),
				depth+1,
				false,
			)
			publication = fmt.Sprintf("%sif %s.%s != nil {\n%s\n%s}",
				Tabs(depth), source, Goify(n, true), publication, Tabs(depth))
			publications = append(publications, publication)
			return nil
		})
	}
	return strings.Join(publications, "\n")
}

// Publicizer publicizes a single attribute based on the type.
func Publicizer(att *design.AttributeExpr, sourceField, targetField string, dereference bool, depth int, init bool) string {
	var publication string
	data := map[string]interface{}{
		"sourceField": sourceField,
		"targetField": targetField,
		"depth":       depth,
		"att":         att,
		"dereference": dereference,
		"init":        init,
	}
	switch {
	case design.IsPrimitive(att.Type):
		publication = RunTemplate(simplePublicizeT, data)
	case design.IsObject(att.Type):
		if _, ok := att.Type.(*design.MediaTypeExpr); ok {
			publication = RunTemplate(recursivePublicizeT, data)
		} else if _, ok := att.Type.(*design.UserTypeExpr); ok {
			publication = RunTemplate(recursivePublicizeT, data)
		} else {
			publication = RunTemplate(objectPublicizeT, data)
		}
	case design.IsArray(att.Type):
		// If the array element is primitive type, we can simply copy the elements over (i.e) []string
		ar := design.AsArray(att.Type)
		if design.IsObject(ar.ElemType.Type) {
			data["elemType"] = ar.ElemType
			publication = RunTemplate(arrayPublicizeT, data)
		} else {
			publication = RunTemplate(simplePublicizeT, data)
		}
	case design.IsMap(att.Type):
		m := design.AsMap(att.Type)
		if design.IsObject(m.KeyType.Type) || design.IsObject(m.ElemType.Type) {
			data["keyType"] = m.KeyType
			data["elemType"] = m.ElemType
			publication = RunTemplate(hashPublicizeT, data)
		} else {
			publication = RunTemplate(simplePublicizeT, data)
		}
	}
	return publication
}

const (
	simplePublicizeTmpl = `{{ tabs .depth }}{{ .targetField }} {{ if .init }}:{{ end }}= {{ if .dereference }}*{{ end }}{{ .sourceField }}`

	recursivePublicizeTmpl = `{{ tabs .depth }}{{ .targetField }} {{ if .init }}:{{ end }}= {{ .sourceField }}.Publicize()`

	objectPublicizeTmpl = `{{ tabs .depth }}{{ .targetField }} = &{{ gotypedef .att .depth true false }}{}
{{ recursivePublicizer .att .sourceField .targetField .depth }}`

	arrayPublicizeTmpl = `{{ tabs .depth }}{{ .targetField }} {{ if .init }}:{{ end }}= make({{ gotyperef .att.Type .att.AllRequired .depth false }}, len({{ .sourceField }})){{/*
*/}}{{ $i := printf "%s%d" "i" .depth }}{{ $elem := printf "%s%d" "elem" .depth }}
{{ tabs .depth }}for {{ $i }}, {{ $elem }} := range {{ .sourceField }} {
{{ tabs .depth }}{{ publicizer .elemType $elem (printf "%s[%s]" .targetField $i) .dereference (add .depth 1) false }}
{{ tabs .depth }}}`

	hashPublicizeTmpl = `{{ tabs .depth }}{{ .targetField }} {{ if .init }}:{{ end }}= make({{ gotyperef .att.Type .att.AllRequired .depth false }}, len({{ .sourceField }})){{/*
*/}}{{ $k := printf "%s%d" "k" .depth }}{{ $v := printf "%s%d" "v" .depth }}
{{ tabs .depth }}for {{ $k }}, {{ $v }} := range {{ .sourceField }} {
{{ $pubk := printf "%s%s" "pub" $k }}{{ $pubv := printf "%s%s" "pub" $v }}{{/*
*/}}{{ tabs .depth }}{{ publicizer .keyType $k $pubk .dereference (add .depth 1) true }}
{{ tabs .depth }}{{ publicizer .elemType $v $pubv .dereference (add .depth 1) true }}
{{ tabs .depth }}	{{ printf "%s[%s]" .targetField $pubk }} = {{ $pubv }}
{{ tabs .depth }}}`
)
