package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type invariants struct {
	Cooldowns []string `json:"cooldowns,omitempty"`
	Logging   []string `json:"logging,omitempty"`
	Journal   []string `json:"journal,omitempty"`
}

type reportEntry struct {
	File          string      `json:"file"`
	Function      string      `json:"function"`
	Line          int         `json:"line"`
	DeliveryKinds []string    `json:"deliveryKinds"`
	EffectTypes   []string    `json:"effectTypes,omitempty"`
	TriggerTypes  []string    `json:"triggerTypes,omitempty"`
	Invariants    *invariants `json:"invariants,omitempty"`
}

type functionInfo struct {
	filePath      string
	line          int
	name          string
	receiverName  string
	direct        bool
	deliveryKinds map[string]struct{}
	effectTypes   map[string]struct{}
	triggerTypes  map[string]struct{}
	cooldowns     map[string]struct{}
	loggingCalls  map[string]struct{}
	journalCalls  map[string]struct{}
	calls         map[string]struct{}
}

func newFunctionInfo(name, filePath, receiver string, line int) *functionInfo {
	return &functionInfo{
		filePath:      filePath,
		line:          line,
		name:          name,
		receiverName:  receiver,
		deliveryKinds: make(map[string]struct{}),
		effectTypes:   make(map[string]struct{}),
		triggerTypes:  make(map[string]struct{}),
		cooldowns:     make(map[string]struct{}),
		loggingCalls:  make(map[string]struct{}),
		journalCalls:  make(map[string]struct{}),
		calls:         make(map[string]struct{}),
	}
}

func (fi *functionInfo) addKind(kind string, direct bool) {
	if kind == "" {
		return
	}
	if _, exists := fi.deliveryKinds[kind]; !exists {
		fi.deliveryKinds[kind] = struct{}{}
	}
	if direct {
		fi.direct = true
	}
}

func (fi *functionInfo) addEffectType(expr string) {
	if expr == "" {
		return
	}
	fi.effectTypes[expr] = struct{}{}
}

func (fi *functionInfo) addTriggerType(expr string) {
	if expr == "" {
		return
	}
	fi.triggerTypes[expr] = struct{}{}
}

func (fi *functionInfo) addCooldown(label string) {
	if label == "" {
		return
	}
	fi.cooldowns[label] = struct{}{}
}

func (fi *functionInfo) addLogging(label string) {
	if label == "" {
		return
	}
	fi.loggingCalls[label] = struct{}{}
}

func (fi *functionInfo) addJournal(label string) {
	if label == "" {
		return
	}
	fi.journalCalls[label] = struct{}{}
}

func (fi *functionInfo) addCall(name string) {
	if name == "" {
		return
	}
	fi.calls[name] = struct{}{}
}

func (fi *functionInfo) mergeFrom(other *functionInfo) bool {
	if other == nil || other == fi {
		return false
	}
	changed := false
	for kind := range other.deliveryKinds {
		if _, exists := fi.deliveryKinds[kind]; !exists {
			fi.deliveryKinds[kind] = struct{}{}
			changed = true
		}
	}
	for t := range other.effectTypes {
		if _, exists := fi.effectTypes[t]; !exists {
			fi.effectTypes[t] = struct{}{}
			changed = true
		}
	}
	for t := range other.triggerTypes {
		if _, exists := fi.triggerTypes[t]; !exists {
			fi.triggerTypes[t] = struct{}{}
			changed = true
		}
	}
	for c := range other.cooldowns {
		if _, exists := fi.cooldowns[c]; !exists {
			fi.cooldowns[c] = struct{}{}
			changed = true
		}
	}
	for l := range other.loggingCalls {
		if _, exists := fi.loggingCalls[l]; !exists {
			fi.loggingCalls[l] = struct{}{}
			changed = true
		}
	}
	for j := range other.journalCalls {
		if _, exists := fi.journalCalls[j]; !exists {
			fi.journalCalls[j] = struct{}{}
			changed = true
		}
	}
	if changed {
		fi.direct = fi.direct || other.direct
	}
	return changed
}

func main() {
	var rootFlag string
	var jsonFlag string
	var csvFlag string

	flag.StringVar(&rootFlag, "root", "", "Repository root path (auto-detected when empty)")
	flag.StringVar(&jsonFlag, "json", "", "Path to write JSON output (defaults to <root>/effects_producer_map.json)")
	flag.StringVar(&csvFlag, "csv", "", "Optional path to write CSV output")
	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	root := rootFlag
	if root == "" {
		root, err = findRepoRoot(cwd)
		if err != nil {
			panic(err)
		}
	}

	serverDir := filepath.Join(root, "server")
	if _, err := os.Stat(serverDir); err != nil {
		panic(fmt.Errorf("failed to locate server directory: %w", err))
	}

	infos, err := scanServer(serverDir, root)
	if err != nil {
		panic(err)
	}

	reports := buildReports(infos)

	if jsonFlag == "" {
		jsonFlag = filepath.Join(root, "effects_producer_map.json")
	}
	if err := writeJSON(jsonFlag, reports); err != nil {
		panic(err)
	}

	if csvFlag != "" {
		if err := writeCSV(csvFlag, reports); err != nil {
			panic(err)
		}
	}
}

func findRepoRoot(start string) (string, error) {
	dir := start
	for {
		if dir == "" || dir == "." {
			dir = filepath.Clean(dir)
		}
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not locate repository root (package.json not found)")
		}
		dir = parent
	}
}

func scanServer(serverDir, repoRoot string) (map[string]*functionInfo, error) {
	fset := token.NewFileSet()
	infos := make(map[string]*functionInfo)

	err := filepath.WalkDir(serverDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}

		importAliases := map[string]string{}
		for _, imp := range file.Imports {
			alias := ""
			if imp.Name != nil {
				alias = imp.Name.Name
			}
			if alias == "" {
				alias = filepath.Base(strings.Trim(imp.Path.Value, "\""))
			}
			importAliases[alias] = strings.Trim(imp.Path.Value, "\"")
		}

		relPath, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Body == nil {
				continue
			}
			if len(fn.Recv.List) == 0 || len(fn.Recv.List[0].Names) == 0 {
				continue
			}
			recvName := fn.Recv.List[0].Names[0].Name
			star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			ident, ok := star.X.(*ast.Ident)
			if !ok || ident.Name != "World" {
				continue
			}

			line := fset.Position(fn.Pos()).Line
			infoKey := fn.Name.Name
			info := infos[infoKey]
			if info == nil {
				info = newFunctionInfo(fn.Name.Name, relPath, recvName, line)
				infos[infoKey] = info
			} else {
				// prefer earliest line/file when duplicates occur
				if line < info.line {
					info.line = line
					info.filePath = relPath
				}
			}

			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.AssignStmt:
					if len(node.Lhs) == 1 && len(node.Rhs) == 1 {
						if sel, ok := node.Lhs[0].(*ast.SelectorExpr); ok {
							if recvIdent, ok := sel.X.(*ast.Ident); ok && recvIdent.Name == recvName && sel.Sel.Name == "effects" {
								if call, ok := node.Rhs[0].(*ast.CallExpr); ok {
									if fnIdent, ok := call.Fun.(*ast.Ident); ok && fnIdent.Name == "append" {
										info.addKind("active-effect", true)
									}
								}
							}
						}
					}
				case *ast.CallExpr:
					if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
						if recvIdent, ok := sel.X.(*ast.Ident); ok {
							if recvIdent.Name == recvName {
								info.addCall(sel.Sel.Name)
								switch sel.Sel.Name {
								case "QueueEffectTrigger":
									info.addKind("trigger", true)
									info.addJournal("QueueEffectTrigger")
								case "applyEffectHitActor", "applyEffectHitNPC", "applyEffectHitPlayer":
									info.addKind("direct-application", true)
								case "cooldownReady":
									info.addCooldown("cooldownReady")
								}
								if strings.HasPrefix(sel.Sel.Name, "SetEffect") || sel.Sel.Name == "purgeEntityPatches" || sel.Sel.Name == "extendAttachedEffect" || sel.Sel.Name == "expireAttachedEffect" {
									info.addJournal(sel.Sel.Name)
								}
							}
						}
						if ident, ok := sel.X.(*ast.Ident); ok {
							if pkgPath, ok := importAliases[ident.Name]; ok && strings.Contains(pkgPath, "/logging") {
								info.addLogging(fmt.Sprintf("%s.%s", ident.Name, sel.Sel.Name))
							}
						}
					}
				case *ast.CompositeLit:
					typeName := compositeTypeName(node.Type)
					switch typeName {
					case "Effect":
						effectTypeExpr := findKeyValueExpr(node.Elts, "Type")
						if effectTypeExpr != "" {
							info.addEffectType(effectTypeExpr)
						}
					case "EffectTrigger":
						triggerTypeExpr := findKeyValueExpr(node.Elts, "Type")
						if triggerTypeExpr != "" {
							info.addTriggerType(triggerTypeExpr)
						}
					}
				}
				return true
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	propagate(infos)
	return infos, nil
}

func propagate(infos map[string]*functionInfo) {
	if len(infos) == 0 {
		return
	}

	changed := true
	for changed {
		changed = false
		for name, info := range infos {
			if info == nil {
				continue
			}
			if info.direct && len(info.deliveryKinds) > 0 {
				// already a producer; ensure we treat as such
			}
			for call := range info.calls {
				if call == name {
					continue
				}
				if target, ok := infos[call]; ok {
					if len(target.deliveryKinds) == 0 && !target.direct {
						continue
					}
					if info.mergeFrom(target) {
						changed = true
					}
				}
			}
		}
	}
}

func buildReports(infos map[string]*functionInfo) []reportEntry {
	entries := make([]reportEntry, 0, len(infos))
	for _, info := range infos {
		if info == nil {
			continue
		}
		if len(info.deliveryKinds) == 0 {
			continue
		}
		entry := reportEntry{
			File:     filepath.ToSlash(info.filePath),
			Function: info.name,
			Line:     info.line,
		}
		entry.DeliveryKinds = sortedKeys(info.deliveryKinds)
		entry.EffectTypes = sortedKeys(info.effectTypes)
		entry.TriggerTypes = sortedKeys(info.triggerTypes)
		inv := invariants{
			Cooldowns: sortedKeys(info.cooldowns),
			Logging:   sortedKeys(info.loggingCalls),
			Journal:   sortedKeys(info.journalCalls),
		}
		if len(inv.Cooldowns) > 0 || len(inv.Logging) > 0 || len(inv.Journal) > 0 {
			entry.Invariants = &inv
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].File == entries[j].File {
			return entries[i].Line < entries[j].Line
		}
		return entries[i].File < entries[j].File
	})
	return entries
}

func writeJSON(path string, entries []reportEntry) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	return nil
}

func writeCSV(path string, entries []reportEntry) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"file", "function", "line", "deliveryKinds", "effectTypes", "triggerTypes", "cooldowns", "logging", "journal"}
	if err := writer.Write(header); err != nil {
		return err
	}

	for _, entry := range entries {
		line := []string{
			entry.File,
			entry.Function,
			fmt.Sprintf("%d", entry.Line),
			strings.Join(entry.DeliveryKinds, "|"),
			strings.Join(entry.EffectTypes, "|"),
			strings.Join(entry.TriggerTypes, "|"),
			strings.Join(nilIfNil(entry.Invariants, func(inv *invariants) []string { return inv.Cooldowns }), "|"),
			strings.Join(nilIfNil(entry.Invariants, func(inv *invariants) []string { return inv.Logging }), "|"),
			strings.Join(nilIfNil(entry.Invariants, func(inv *invariants) []string { return inv.Journal }), "|"),
		}
		if err := writer.Write(line); err != nil {
			return err
		}
	}

	return writer.Error()
}

type sliceExtractor func(*invariants) []string

func nilIfNil(inv *invariants, fn sliceExtractor) []string {
	if inv == nil || fn == nil {
		return nil
	}
	return fn(inv)
}

func compositeTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.SelectorExpr:
		return t.Sel.Name
	}
	return ""
}

func findKeyValueExpr(elts []ast.Expr, key string) string {
	for _, elt := range elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		ident, ok := kv.Key.(*ast.Ident)
		if !ok || ident.Name != key {
			continue
		}
		return exprString(kv.Value)
	}
	return ""
}

func exprString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	var sb strings.Builder
	if err := format.Node(&sb, token.NewFileSet(), expr); err != nil {
		return ""
	}
	return sb.String()
}

func sortedKeys(m map[string]struct{}) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
