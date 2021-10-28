package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
)

var (
	structCommentCollection  = "mongowrapper:collection"
	structCommentAggregation = "mongowrapper:aggregation"
	bodyComment              = "in: body"
)

type Parser struct {
	In, Dir, PkgPath, PkgName string
	isDir                     bool
	Collections, Aggregations []*DataView
	Structs                   []*StructInfo
	Decls                     map[string]*ast.StructType
}

type StructInfo struct {
	Name         string
	Body         *ast.StructType
	BodyTypeName string
	Fields       []Field
}
type Field struct {
	Prop, Type, JsonProp, JsonPath, BsonProp, BsonPath, GoPath, Ns, NsShort, NsCompact, Tag string
	Validations                                                                             map[string]string
	IsId                                                                                    bool
}
type DataView struct {
	Typ, Name, IdGoPath string
	Fields              []Field
	HasId               bool
}

type visitor struct {
	*Parser

	name string
}

func NewParser(in string) *Parser {
	root, _ := os.Getwd()
	fin := path.Join(root, in)
	fInfo, err := os.Stat(fin)
	if err != nil {
		fmt.Printf("Error parsing %v: %v\n", in, err)
		os.Exit(1)
	}

	p := &Parser{
		In: fin, isDir: fInfo.IsDir(),
		Structs:     make([]*StructInfo, 0, 50),
		Collections: make([]*DataView, 0, 50),
		Decls:       make(map[string]*ast.StructType, 200),
	}

	if fInfo.IsDir() {
		p.Dir = fin
	} else {
		p.Dir = filepath.Dir(fin)
	}
	return p
}

func (p *Parser) Parse() error {
	var err error
	if p.PkgPath, err = GetPkgPath(p.In, p.isDir); err != nil {
		return err
	}

	fset := token.NewFileSet()
	if p.isDir {
		log.Printf("parse dir: %s", p.Dir)
		packages, err := parser.ParseDir(fset, p.Dir, excludeTestFiles, parser.ParseComments)
		if err != nil {
			return err
		}

		if len(packages) != 1 {
			return fmt.Errorf("only one package in directory supported\n")
		}

		for _, pckg := range packages {
			for _, f := range pckg.Files {
				FillDecls(f, p.Decls)
			}
			ast.Walk(&visitor{Parser: p}, pckg)
		}
	} else {
		log.Printf("parse file: %s", p.In)
		f, err := parser.ParseFile(fset, p.In, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		FillDecls(f, p.Decls)
		ast.Walk(&visitor{Parser: p}, f)
	}

	return nil
}

func (v *visitor) Visit(n ast.Node) (w ast.Visitor) {
	switch n := n.(type) {
	case *ast.Package:
		return v
	case *ast.File:
		v.PkgName = n.Name.String()
		return v

	case *ast.GenDecl:
		args := v.needType(n.Doc, structCommentCollection)
		if len(args) == 0 {
			args = v.needType(n.Doc, structCommentAggregation)
		}

		if len(args) > 0 {
			for _, nc := range n.Specs {
				switch nct := nc.(type) {
				case *ast.TypeSpec:
					nct.Doc = n.Doc
				}
			}
		}

		return v
	case *ast.TypeSpec:
		args := v.needType(n.Doc, structCommentCollection)
		if len(args) > 1 {

			v.name = n.Name.String()
			fmt.Printf("parsing collection %s\n", v.name)

			fields := make([]Field, 0, 100)
			deep(n.Type, Field{}, &fields)
			hasId, idGoPath := false, ""
			for _, f := range fields {
				if f.IsId {
					hasId = true
					idGoPath = f.GoPath
					break
				}
			}
			v.Parser.Collections = append(v.Parser.Collections, &DataView{
				Typ:      v.name,
				Name:     args[1],
				Fields:   fields,
				HasId:    hasId,
				IdGoPath: idGoPath,
			})
		}

		args = v.needType(n.Doc, structCommentAggregation)
		if len(args) > 1 {
			v.name = n.Name.String()
			fmt.Printf("parsing aggregation %s\n", v.name)
			fields := make([]Field, 0, 100)
			deep(n.Type, Field{}, &fields)
			v.Parser.Aggregations = append(v.Parser.Aggregations, &DataView{
				Typ:    v.name,
				Name:   args[1],
				Fields: fields,
			})
		}

		return nil
	case *ast.StructType:
		//v.StructNames = append(v.StructNames, &structProps{Name: v.name})
		return nil
	}
	return nil
}

func (p *Parser) needType(comments *ast.CommentGroup, reqComment string) (arguments []string) {
	if comments == nil {
		return
	}

	for _, v := range comments.List {
		comment := v.Text

		if len(comment) > 2 {
			switch comment[1] {
			case '/':
				// -style comment (no newline at the end)
				comment = comment[2:]
			case '*':
				/*-style comment */
				comment = comment[2 : len(comment)-2]
			}
		}

		for _, comment := range strings.Split(comment, "\n") {

			comment = strings.TrimSpace(comment)

			if strings.HasPrefix(comment, reqComment) {
				data := strings.FieldsFunc(comment, func(r rune) bool {
					return r == ' '
				})
				return data
			}
		}
	}

	return
}

func excludeTestFiles(fi os.FileInfo) bool {
	return !strings.HasSuffix(fi.Name(), "_test.go")
}

func deep(n ast.Node, f Field, fields *[]Field) {
	switch n := n.(type) {
	case *ast.TypeSpec:
		switch ts := n.Type.(type) {
		case *ast.StarExpr:
			deep(ts.X, f, fields)
		case *ast.StructType:
			deep(ts, f, fields)
		default:
			return
		}
	case *ast.GenDecl:
		for _, nc := range n.Specs {
			switch nct := nc.(type) {
			case *ast.TypeSpec:
				f.Prop = nct.Name.Name
				deep(nc, f, fields)

			}
		}
	case *ast.StructType:
		for _, field := range n.Fields.List {
			fi := *(&f)
			fi.Prop = field.Names[0].Name
			if len(fi.BsonPath) > 0 {
				fi.BsonPath += "."
			}
			if len(fi.JsonPath) > 0 {
				fi.JsonPath += "."
			}
			fi.BsonProp = GetTag(field.Tag, "bson", field.Names[0].Name, 0)
			fi.JsonProp = GetTag(field.Tag, "json", field.Names[0].Name, 0)
			if field.Tag != nil {
				fi.Tag = field.Tag.Value
			}
			switch ss := field.Type.(type) {
			case *ast.StructType:
				fi.BsonPath, fi.JsonPath, fi.GoPath = fi.BsonPath+fi.BsonProp, fi.JsonPath+fi.JsonProp, fi.GoPath+field.Names[0].Name
				deep(ss, fi, fields)
			case *ast.StarExpr:
				if ident, ok := ss.X.(*ast.Ident); ok {
					if ident.Obj != nil {
						if ts, ok := ident.Obj.Decl.(*ast.TypeSpec); ok {
							fi.BsonPath, fi.JsonPath, fi.Ns, fi.GoPath = fi.BsonPath+fi.BsonProp, fi.JsonPath+fi.JsonProp, fi.Ns+"."+field.Names[0].Name, fi.GoPath+field.Names[0].Name
							deep(ts.Type, fi, fields)
						}
					} else {
						deep(field.Type, fi, fields)
					}
				}
			default:
				deep(field.Type, fi, fields)
			}

		}
	case *ast.Ident:
		var typ string
		if n.Obj != nil {
			typ = n.Obj.Name
		} else {
			typ = n.Name
		}
		ns := f.Ns + "." + f.Prop
		idx := strings.IndexByte(ns, byte('.'))
		f := &Field{
			Prop:        f.Prop,
			GoPath:      f.GoPath + f.Prop,
			JsonProp:    f.JsonProp,
			JsonPath:    f.JsonPath + f.JsonProp,
			BsonProp:    f.BsonProp,
			BsonPath:    f.BsonPath + f.BsonProp,
			Type:        typ,
			Ns:          ns,
			NsShort:     ns[idx+1:],
			NsCompact:   strings.Replace(ns, ".", "", -1),
			Validations: getValidateRules(f.Tag),
		}
		*fields = append(*fields, *f)
	case *ast.SelectorExpr:
		var typ string
		if e, ok := n.X.(*ast.Ident); ok {
			if e.Name != "" {
				typ = e.Name + "."
			}
		}
		ns := f.GoPath + "." + f.Prop
		idx := strings.IndexByte(ns, byte('.'))
		f := &Field{
			IsId:        f.BsonProp == "_id",
			Prop:        f.Prop,
			GoPath:      f.GoPath + f.Prop,
			JsonProp:    f.JsonProp,
			JsonPath:    f.JsonPath + f.JsonProp,
			BsonProp:    f.BsonProp,
			BsonPath:    f.BsonPath + f.BsonProp,
			Type:        typ + n.Sel.Name,
			Ns:          ns,
			NsShort:     ns[idx+1:],
			NsCompact:   strings.Replace(ns, ".", "", -1),
			Validations: getValidateRules(f.Tag),
		}
		*fields = append(*fields, *f)
		break
	case *ast.StarExpr:
		deep(n.X, f, fields)
		break
	default:
		break
	}
}

func getValidateRules(tag string) map[string]string {
	if tag == "" {
		return nil
	}
	m := make(map[string]string, 5)
	keys := strings.Split(reflect.StructTag(tag[1:len(tag)-1]).Get("validate"), ",")
	for _, k := range keys {
		if k == "" || k == "required" {
			continue
		}
		idx := strings.IndexByte(k, '=')
		if idx > 0 {
			m[k[:idx]] = k[idx+1:]
		} else {
			m[k] = ""
		}
	}
	return m
}
