package pipeline

import (
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"
)

type contractDefinition struct {
	ID     string
	Spawn  payloadBinding
	Update payloadBinding
	End    payloadBinding
}

type payloadBinding struct {
	TypeName    string
	IsNoPayload bool
}

type tsField struct {
	Name     string
	Type     string
	Optional bool
}

type tsInterface struct {
	Name   string
	Fields []tsField
}

type tsAlias struct {
	Name   string
	Target string
}

type tsDeclarations struct {
	Interfaces []tsInterface
	Aliases    []tsAlias
}

func loadContractMetadata(contractsDir, registryPath string) ([]contractDefinition, tsDeclarations, error) {
	pkg, err := loadContractsPackage(contractsDir)
	if err != nil {
		return nil, tsDeclarations{}, err
	}

	aliasTargets := collectAliasTargets(pkg)
	translator := newTypeTranslator(pkg, aliasTargets)

	definitions, err := parseRegistryDefinitions(pkg, registryPath, translator)
	if err != nil {
		return nil, tsDeclarations{}, err
	}

	decls := translator.declarations()

	return definitions, decls, nil
}

func loadContractsPackage(dir string) (*packages.Package, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("effectsgen: unable to resolve contracts directory: %w", err)
	}

	modRoot, err := findModuleRoot(absDir)
	if err != nil {
		return nil, err
	}

	relPath, err := filepath.Rel(modRoot, absDir)
	if err != nil {
		return nil, fmt.Errorf("effectsgen: failed computing package path: %w", err)
	}

	pattern := "./" + filepath.ToSlash(relPath)
	cfg := &packages.Config{
		Dir:  modRoot,
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax | packages.NeedFiles | packages.NeedModule | packages.NeedDeps,
	}

	pkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		return nil, fmt.Errorf("effectsgen: failed loading contracts package: %w", err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("effectsgen: package load returned no results for %s", pattern)
	}
	if len(pkgs) > 1 {
		return nil, fmt.Errorf("effectsgen: expected a single package for %s, got %d", pattern, len(pkgs))
	}

	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		return nil, fmt.Errorf("effectsgen: package load reported errors: %v", pkg.Errors[0])
	}

	return pkg, nil
}

func findModuleRoot(start string) (string, error) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("effectsgen: failed probing %s: %w", dir, err)
		}

		next := filepath.Dir(dir)
		if next == dir {
			return "", fmt.Errorf("effectsgen: unable to locate go.mod for %s", start)
		}
		dir = next
	}
}

func collectAliasTargets(pkg *packages.Package) map[string]types.Type {
	targets := make(map[string]types.Type)
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Assign == token.NoPos {
					continue
				}
				obj, ok := pkg.TypesInfo.Defs[ts.Name]
				if !ok {
					continue
				}
				typeName, ok := obj.(*types.TypeName)
				if !ok || !typeName.IsAlias() {
					continue
				}
				target := pkg.TypesInfo.TypeOf(ts.Type)
				if target == nil {
					continue
				}
				targets[typeName.Name()] = target
			}
		}
	}
	return targets
}

func parseRegistryDefinitions(pkg *packages.Package, registryPath string, translator *typeTranslator) ([]contractDefinition, error) {
	absRegistry, err := filepath.Abs(registryPath)
	if err != nil {
		return nil, fmt.Errorf("effectsgen: unable to resolve registry path: %w", err)
	}

	file := findFileByPath(pkg, absRegistry)
	if file == nil {
		return nil, fmt.Errorf("effectsgen: registry file %s not part of package", registryPath)
	}

	registryObj := pkg.Types.Scope().Lookup("Registry")
	if registryObj == nil {
		return nil, fmt.Errorf("effectsgen: contract package missing Registry type")
	}
	registryType := registryObj.Type()

	var definitions []contractDefinition
	found := false

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			if len(vs.Names) == 0 || len(vs.Values) == 0 {
				continue
			}
			for idx, name := range vs.Names {
				obj, ok := pkg.TypesInfo.Defs[name]
				if !ok {
					continue
				}
				variable, ok := obj.(*types.Var)
				if !ok {
					continue
				}
				if !types.Identical(variable.Type(), registryType) {
					continue
				}

				valueExpr := selectValueExpression(vs, idx)
				lit, ok := valueExpr.(*ast.CompositeLit)
				if !ok {
					return nil, fmt.Errorf("effectsgen: registry variable %s must be initialised with a composite literal", name.Name)
				}

				regs, err := decodeRegistryEntries(pkg, lit, translator)
				if err != nil {
					return nil, err
				}
				definitions = append(definitions, regs...)
				found = true
			}
		}
	}

	if !found {
		return nil, fmt.Errorf("effectsgen: no Registry declarations found in %s", registryPath)
	}

	sort.Slice(definitions, func(i, j int) bool {
		return definitions[i].ID < definitions[j].ID
	})

	return definitions, nil
}

func findFileByPath(pkg *packages.Package, path string) *ast.File {
	cleaned := filepath.Clean(path)
	for _, file := range pkg.Syntax {
		pos := pkg.Fset.Position(file.Package)
		if filepath.Clean(pos.Filename) == cleaned {
			return file
		}
	}
	return nil
}

func selectValueExpression(vs *ast.ValueSpec, index int) ast.Expr {
	if len(vs.Values) == len(vs.Names) {
		return vs.Values[index]
	}
	return vs.Values[0]
}

func decodeRegistryEntries(pkg *packages.Package, lit *ast.CompositeLit, translator *typeTranslator) ([]contractDefinition, error) {
	defs := make([]contractDefinition, 0, len(lit.Elts))
	for _, elt := range lit.Elts {
		entry, ok := elt.(*ast.CompositeLit)
		if !ok {
			return nil, fmt.Errorf("effectsgen: registry entry must be a composite literal")
		}

		def := contractDefinition{}
		for _, element := range entry.Elts {
			kv, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			keyIdent, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}
			switch keyIdent.Name {
			case "ID":
				id, err := evaluateStringConstant(pkg, kv.Value)
				if err != nil {
					return nil, err
				}
				def.ID = id
			case "Spawn":
				binding, err := resolvePayloadBinding(pkg, kv.Value, translator)
				if err != nil {
					return nil, err
				}
				def.Spawn = binding
			case "Update":
				binding, err := resolvePayloadBinding(pkg, kv.Value, translator)
				if err != nil {
					return nil, err
				}
				def.Update = binding
			case "End":
				binding, err := resolvePayloadBinding(pkg, kv.Value, translator)
				if err != nil {
					return nil, err
				}
				def.End = binding
			}
		}

		if def.ID == "" {
			return nil, fmt.Errorf("effectsgen: registry entry missing ID value")
		}
		defs = append(defs, def)
	}
	return defs, nil
}

func evaluateStringConstant(pkg *packages.Package, expr ast.Expr) (string, error) {
	tv, ok := pkg.TypesInfo.Types[expr]
	if !ok || tv.Value == nil {
		return "", fmt.Errorf("effectsgen: expected string constant, got %T", expr)
	}
	if tv.Value.Kind() != constant.String {
		return "", fmt.Errorf("effectsgen: expected string constant, got %s", tv.Value.Kind())
	}
	return constant.StringVal(tv.Value), nil
}

func resolvePayloadBinding(pkg *packages.Package, expr ast.Expr, translator *typeTranslator) (payloadBinding, error) {
	if ident, ok := expr.(*ast.Ident); ok && ident.Name == "NoPayload" {
		return payloadBinding{IsNoPayload: true, TypeName: "null"}, nil
	}

	typeName, err := resolveTypeName(pkg, expr)
	if err != nil {
		return payloadBinding{}, err
	}

	tsName, err := translator.typeNameReference(typeName)
	if err != nil {
		return payloadBinding{}, err
	}

	return payloadBinding{TypeName: tsName}, nil
}

func resolveTypeName(pkg *packages.Package, expr ast.Expr) (*types.TypeName, error) {
	switch e := expr.(type) {
	case *ast.CallExpr:
		return resolveTypeName(pkg, e.Fun)
	case *ast.ParenExpr:
		return resolveTypeName(pkg, e.X)
	case *ast.StarExpr:
		return resolveTypeName(pkg, e.X)
	case *ast.Ident:
		obj, ok := pkg.TypesInfo.Uses[e]
		if !ok {
			return nil, fmt.Errorf("effectsgen: unable to resolve identifier %s", e.Name)
		}
		typeName, ok := obj.(*types.TypeName)
		if !ok {
			return nil, fmt.Errorf("effectsgen: %s does not reference a type", e.Name)
		}
		return typeName, nil
	default:
		return nil, fmt.Errorf("effectsgen: unsupported payload expression %T", expr)
	}
}

type typeTranslator struct {
	pkg          *packages.Package
	aliasTargets map[string]types.Type
	interfaces   map[string]tsInterface
	aliases      map[string]string
	structStack  map[string]bool
}

func newTypeTranslator(pkg *packages.Package, aliasTargets map[string]types.Type) *typeTranslator {
	return &typeTranslator{
		pkg:          pkg,
		aliasTargets: aliasTargets,
		interfaces:   make(map[string]tsInterface),
		aliases:      make(map[string]string),
		structStack:  make(map[string]bool),
	}
}

func (t *typeTranslator) namedTypeReference(named *types.Named) (string, error) {
	return t.typeReference(named)
}

func (t *typeTranslator) typeNameReference(typeName *types.TypeName) (string, error) {
	if named, ok := typeName.Type().(*types.Named); ok {
		return t.namedTypeReference(named)
	}
	if target, ok := t.aliasTargets[typeName.Name()]; ok {
		tsName, err := t.typeReference(target)
		if err != nil {
			return "", err
		}
		t.aliases[typeName.Name()] = tsName
		return typeName.Name(), nil
	}
	return "", fmt.Errorf("effectsgen: %s is not a named type", typeName.Name())
}

func (t *typeTranslator) typeReference(typ types.Type) (string, error) {
	switch tt := typ.(type) {
	case *types.Pointer:
		return t.typeReference(tt.Elem())
	case *types.Named:
		name := tt.Obj().Name()
		if tt.Obj().Pkg() != nil && tt.Obj().Pkg().Path() != t.pkg.PkgPath {
			return t.typeReference(tt.Underlying())
		}
		if tt.Obj().IsAlias() {
			target := t.aliasTargets[name]
			if target == nil {
				target = tt.Underlying()
			}
			targetName, err := t.typeReference(target)
			if err != nil {
				return "", err
			}
			if existing, ok := t.aliases[name]; ok {
				if existing != targetName {
					return "", fmt.Errorf("effectsgen: conflicting alias target for %s", name)
				}
			} else {
				t.aliases[name] = targetName
			}
			return name, nil
		}

		if _, ok := tt.Underlying().(*types.Struct); ok {
			if _, exists := t.interfaces[name]; !exists {
				if err := t.emitStructInterface(name, tt.Underlying().(*types.Struct)); err != nil {
					return "", err
				}
			}
			return name, nil
		}

		return t.typeReference(tt.Underlying())
	case *types.Struct:
		return "", fmt.Errorf("effectsgen: anonymous struct types are not supported in payload declarations")
	case *types.Slice:
		elem, err := t.typeReference(tt.Elem())
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("ReadonlyArray<%s>", elem), nil
	case *types.Array:
		elem, err := t.typeReference(tt.Elem())
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("ReadonlyArray<%s>", elem), nil
	case *types.Map:
		key, err := t.typeReference(tt.Key())
		if err != nil {
			return "", err
		}
		if key != "string" {
			return "", fmt.Errorf("effectsgen: map keys must be strings, got %s", tt.Key().String())
		}
		value, err := t.typeReference(tt.Elem())
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Readonly<Record<string, %s>>", value), nil
	case *types.Basic:
		switch tt.Kind() {
		case types.Bool:
			return "boolean", nil
		case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
			types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64, types.Uintptr,
			types.Float32, types.Float64:
			return "number", nil
		case types.String:
			return "string", nil
		case types.UnsafePointer:
			return "unknown", nil
		case types.UntypedNil:
			return "null", nil
		default:
			return "", fmt.Errorf("effectsgen: unsupported basic kind %s", tt.Name())
		}
	default:
		return "", fmt.Errorf("effectsgen: unsupported type %T", typ)
	}
}

func (t *typeTranslator) emitStructInterface(name string, st *types.Struct) error {
	if t.structStack[name] {
		return nil
	}
	t.structStack[name] = true

	fields := make([]tsField, 0, st.NumFields())
	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		if field.Embedded() {
			continue
		}
		if !field.Exported() {
			continue
		}

		tag := st.Tag(i)
		jsonName, opts := parseJSONTag(tag)
		if jsonName == "-" {
			continue
		}

		name := jsonName
		if name == "" {
			name = field.Name()
			name = lowerFirst(name)
		}

		optional := hasOmitEmpty(opts)
		effectTag := reflect.StructTag(tag).Get("effect")
		if effectTag != "" {
			for _, part := range strings.Split(effectTag, ",") {
				if strings.TrimSpace(part) == "optional" {
					optional = true
				}
			}
		}

		fieldType := field.Type()
		if ptr, ok := fieldType.(*types.Pointer); ok {
			fieldType = ptr.Elem()
			optional = true
		}

		tsType, err := t.typeReference(fieldType)
		if err != nil {
			return err
		}

		fields = append(fields, tsField{
			Name:     name,
			Type:     tsType,
			Optional: optional,
		})
	}

	t.interfaces[name] = tsInterface{Name: name, Fields: fields}
	delete(t.structStack, name)
	return nil
}

func parseJSONTag(tag string) (string, []string) {
	if tag == "" {
		return "", nil
	}
	value := reflect.StructTag(tag).Get("json")
	if value == "" {
		return "", nil
	}
	parts := strings.Split(value, ",")
	return parts[0], parts[1:]
}

func hasOmitEmpty(opts []string) bool {
	for _, opt := range opts {
		if strings.TrimSpace(opt) == "omitempty" {
			return true
		}
	}
	return false
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func (t *typeTranslator) declarations() tsDeclarations {
	interfaces := make([]tsInterface, 0, len(t.interfaces))
	for _, decl := range t.interfaces {
		fields := make([]tsField, len(decl.Fields))
		copy(fields, decl.Fields)
		interfaces = append(interfaces, tsInterface{Name: decl.Name, Fields: fields})
	}
	sort.Slice(interfaces, func(i, j int) bool {
		return interfaces[i].Name < interfaces[j].Name
	})

	aliases := make([]tsAlias, 0, len(t.aliases))
	for name, target := range t.aliases {
		aliases = append(aliases, tsAlias{Name: name, Target: target})
	}
	sort.Slice(aliases, func(i, j int) bool {
		return aliases[i].Name < aliases[j].Name
	})

	return tsDeclarations{Interfaces: interfaces, Aliases: aliases}
}
