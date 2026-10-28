package main

// Source extraction deliberately uses the Go AST, never application constructors,
// a database, network access or mutable global state. It describes JSON decoding
// shape, not service validation or writable database columns.
import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

type schema = map[string]interface{}
type typeDef struct {
	expr ast.Expr
	pkg  string
}
type sourceReader struct {
	types   map[string]typeDef
	schemas map[string]schema
}

func readSourceContract(root string) (map[string]interface{}, error) {
	s := &sourceReader{types: map[string]typeDef{}, schemas: map[string]schema{}}
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, pkg := range []string{"model", "service", "handler", "middleware", "router"} {
		paths, err := filepath.Glob(filepath.Join(root, pkg, "*.go"))
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return nil, err
			}
			files[path] = file
			for _, decl := range file.Decls {
				g, ok := decl.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, spec := range g.Specs {
					if t, ok := spec.(*ast.TypeSpec); ok {
						s.types[pkg+"."+t.Name.Name] = typeDef{t.Type, pkg}
					}
				}
			}
		}
	}
	// Structs with JSON tags are data contracts. Service structs, repositories,
	// DB handles and other implementation objects do not belong in OpenAPI.
	keys := []string{}
	for key, d := range s.types {
		st, ok := d.expr.(*ast.StructType)
		if !ok {
			continue
		}
		for _, field := range st.Fields.List {
			if field.Tag != nil && strings.Contains(field.Tag.Value, "json:") {
				keys = append(keys, key)
				break
			}
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		s.named(key)
	}
	handlers := map[string]interface{}{}
	errorDefinitions := []map[string]interface{}{}
	for path, file := range files {
		if file.Name.Name != "handler" && file.Name.Name != "middleware" && file.Name.Name != "router" {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := ast.NewIdent(file.Name.Name)
			if fn.Recv != nil {
				receiver := fn.Recv.List[0].Type
				if ptr, ok := receiver.(*ast.StarExpr); ok {
					receiver = ptr.X
				}
				name, ok = receiver.(*ast.Ident)
				if !ok {
					continue
				}
			}
			key := name.Name + "." + fn.Name.Name
			locals := map[string]ast.Expr{}
			query := map[string]schema{}
			calls := []string{}
			meta := map[string]interface{}{"source": filepath.ToSlash(filepath.Join("backend", path)), "line": fset.Position(fn.Pos()).Line}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.ValueSpec:
					if v.Type != nil {
						for _, ident := range v.Names {
							locals[ident.Name] = v.Type
						}
					}
				case *ast.CallExpr:
					if id, ok := v.Fun.(*ast.Ident); ok {
						if id.Name == "parseVersion" {
							query["version"] = schema{"type": "integer", "minimum": 1, "x-required": true}
						}
						if id.Name == "queryInt" && len(v.Args) > 1 {
							if key := literal(v.Args[1]); key != "" {
								query[key] = schema{"type": "integer"}
								if len(v.Args) > 2 {
									if d, err := strconv.Atoi(literal(v.Args[2])); err == nil {
										query[key]["default"] = d
									}
								}
							}
						}
					}
					sel, ok := v.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					base, _ := sel.X.(*ast.Ident)
					if base != nil && base.Name == "response" && sel.Sel.Name == "Error" && len(v.Args) >= 4 {
						if code := literal(v.Args[2]); code != "" {
							status := literal(v.Args[1])
							if constant, ok := v.Args[1].(*ast.SelectorExpr); ok {
								statuses := map[string]int{"StatusBadRequest": http.StatusBadRequest, "StatusUnauthorized": http.StatusUnauthorized, "StatusForbidden": http.StatusForbidden, "StatusNotFound": http.StatusNotFound, "StatusConflict": http.StatusConflict, "StatusGone": http.StatusGone, "StatusInternalServerError": http.StatusInternalServerError, "StatusServiceUnavailable": http.StatusServiceUnavailable, "StatusTooManyRequests": http.StatusTooManyRequests, "StatusRequestEntityTooLarge": http.StatusRequestEntityTooLarge}
								if value, ok := statuses[constant.Sel.Name]; ok {
									status = strconv.Itoa(value)
								}
							}
							errorDefinitions = append(errorDefinitions, map[string]interface{}{"code": code, "status": status, "message": literal(v.Args[3]), "handler": key, "source": filepath.ToSlash(filepath.Join("backend", path)), "line": fset.Position(v.Pos()).Line})
						}
					}
					if base != nil && base.Name == "h" {
						calls = append(calls, name.Name+"."+sel.Sel.Name)
					}
					if sel.Sel.Name == "ShouldBindJSON" && len(v.Args) > 0 {
						arg := v.Args[0]
						if ref, ok := arg.(*ast.UnaryExpr); ok {
							arg = ref.X
						}
						if id, ok := arg.(*ast.Ident); ok {
							if typ, ok := locals[id.Name]; ok {
								meta["requestSchema"] = s.expr(typ, "handler")
							}
						}
					}
					if base != nil && base.Name == "c" && (sel.Sel.Name == "Query" || sel.Sel.Name == "DefaultQuery") && len(v.Args) > 0 {
						if key := literal(v.Args[0]); key != "" {
							query[key] = schema{"type": "string"}
						}
					}
				}
				return true
			})
			if len(query) > 0 {
				meta["query"] = query
			}
			if len(calls) > 0 {
				meta["calls"] = calls
			}
			handlers[key] = meta
		}
	}
	sort.Slice(errorDefinitions, func(i, j int) bool {
		a, b := errorDefinitions[i], errorDefinitions[j]
		return fmt.Sprint(a["code"], a["source"], a["line"]) < fmt.Sprint(b["code"], b["source"], b["line"])
	})
	return map[string]interface{}{"handlers": handlers, "schemas": s.schemas, "handlerErrorDefinitions": errorDefinitions}, nil
}

func literal(expr ast.Expr) string {
	if x, ok := expr.(*ast.BasicLit); ok {
		if x.Kind == token.STRING {
			value, _ := strconv.Unquote(x.Value)
			return value
		}
		return x.Value
	}
	return ""
}
func schemaKey(key string) string { return strings.ReplaceAll(key, ".", "_") }
func (s *sourceReader) named(key string) schema {
	if key == "handler.optionalInt" {
		return schema{"type": "integer"}
	}
	name := schemaKey(key)
	if _, ok := s.schemas[name]; !ok {
		d, ok := s.types[key]
		if !ok {
			return schema{"x-go-type": key, "description": "未解析的 Go 类型；未承诺其 JSON 结构"}
		}
		s.schemas[name] = schema{} // break recursive model references
		result := s.expr(d.expr, d.pkg)
		result["x-go-type"] = key
		s.schemas[name] = result
	}
	return schema{"$ref": "#/components/schemas/" + name}
}
func (s *sourceReader) expr(expr ast.Expr, pkg string) schema {
	switch t := expr.(type) {
	case *ast.Ident:
		switch t.Name {
		case "string":
			return schema{"type": "string"}
		case "bool":
			return schema{"type": "boolean"}
		case "int", "int8", "int16", "int32", "int64":
			return schema{"type": "integer"}
		case "uint", "uint8", "uint16", "uint32", "uint64":
			return schema{"type": "integer", "minimum": 0}
		case "float32", "float64":
			return schema{"type": "number"}
		case "any":
			return schema{}
		default:
			return s.named(pkg + "." + t.Name)
		}
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			key := x.Name + "." + t.Sel.Name
			switch key {
			case "uuid.UUID":
				return schema{"type": "string", "format": "uuid"}
			case "time.Time":
				return schema{"type": "string", "format": "date-time"}
			case "json.RawMessage":
				return schema{}
			case "gorm.DeletedAt":
				return schema{"type": "string", "format": "date-time", "nullable": true}
			}
			return s.named(key)
		}
	case *ast.StarExpr:
		child := s.expr(t.X, pkg)
		if _, ok := child["$ref"]; ok {
			return schema{"allOf": []interface{}{child}, "nullable": true}
		}
		child["nullable"] = true
		return child
	case *ast.ArrayType:
		if id, ok := t.Elt.(*ast.Ident); ok && id.Name == "byte" {
			return schema{"type": "string", "format": "byte", "nullable": t.Len == nil}
		}
		return schema{"type": "array", "items": s.expr(t.Elt, pkg), "nullable": t.Len == nil}
	case *ast.MapType:
		return schema{"type": "object", "additionalProperties": s.expr(t.Value, pkg), "nullable": true}
	case *ast.InterfaceType:
		return schema{}
	case *ast.StructType:
		props := map[string]interface{}{}
		required := []string{}
		for _, f := range t.Fields.List {
			if len(f.Names) > 0 {
				continue
			}
			// Flatten anonymous fields, matching encoding/json. Named fields below
			// take precedence over embedded fields (e.g. optional racksPerRow).
			base := s.expr(f.Type, pkg)
			if ref, ok := base["$ref"].(string); ok {
				base = s.schemas[strings.TrimPrefix(ref, "#/components/schemas/")]
			}
			if p, ok := base["properties"].(map[string]interface{}); ok {
				for k, v := range p {
					props[k] = v
				}
			}
		}
		for _, f := range t.Fields.List {
			if len(f.Names) == 0 {
				continue
			}
			tag := reflect.StructTag("")
			if f.Tag != nil {
				unquoted, _ := strconv.Unquote(f.Tag.Value)
				tag = reflect.StructTag(unquoted)
			}
			for _, n := range f.Names {
				if !ast.IsExported(n.Name) {
					continue
				}
				name := strings.Split(tag.Get("json"), ",")[0]
				if name == "-" {
					continue
				}
				if name == "" {
					name = n.Name
				}
				value := s.expr(f.Type, pkg)
				for _, rule := range strings.Split(tag.Get("binding"), ",") {
					if rule == "required" {
						required = append(required, name)
					}
					if strings.HasPrefix(rule, "max=") {
						if size, err := strconv.Atoi(strings.TrimPrefix(rule, "max=")); err == nil && value["type"] == "string" {
							value["maxLength"] = size
						}
					}
				}
				props[name] = value
			}
		}
		result := schema{"type": "object", "properties": props}
		if len(required) > 0 {
			sort.Strings(required)
			result["required"] = required
		}
		return result
	}
	return schema{"description": fmt.Sprintf("未解析的 Go AST 类型 %T", expr)}
}
