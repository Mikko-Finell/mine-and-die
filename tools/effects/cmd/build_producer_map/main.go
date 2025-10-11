package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type functionReport struct {
	File            string   `json:"file"`
	Function        string   `json:"function"`
	Categories      []string `json:"categories"`
	DeliveryKinds   []string `json:"deliveryKinds,omitempty"`
	EffectTypes     []string `json:"effectTypes,omitempty"`
	TriggerTypes    []string `json:"triggerTypes,omitempty"`
	CooldownGuards  []string `json:"cooldownGuards,omitempty"`
	CooldownSources []string `json:"cooldownSources,omitempty"`
	LoggingCalls    []string `json:"loggingCalls,omitempty"`
	JournalCalls    []string `json:"journalCalls,omitempty"`
	Mutations       []string `json:"mutations,omitempty"`
	Notes           []string `json:"notes,omitempty"`
}

type functionInfo struct {
	filePath string
	name     string

	effectTypes  map[string]struct{}
	triggerTypes map[string]struct{}

	cooldownGuards  map[string]struct{}
	cooldownSources map[string]struct{}
	loggingCalls    map[string]struct{}
	journalCalls    map[string]struct{}
	mutations       map[string]struct{}
	categories      map[string]struct{}

	hasEffectLiteral  bool
	hasTriggerLiteral bool
	queueTriggerCall  bool
}

type valueInfo struct {
	effectTypes  []string
	triggerTypes []string
}

func main() {
	rootFlag := flag.String("root", "..", "project root")
	outFlag := flag.String("out", "", "output file (default <root>/effects_producer_map.json)")
	flag.Parse()

	rootAbs, err := filepath.Abs(*rootFlag)
	if err != nil {
		exitErr(fmt.Errorf("resolve root: %w", err))
	}

	outPath := *outFlag
	if outPath == "" {
		outPath = filepath.Join(rootAbs, "effects_producer_map.json")
	} else if !filepath.IsAbs(outPath) {
		outPath = filepath.Join(rootAbs, outPath)
	}

	reports, err := analyze(rootAbs)
	if err != nil {
		exitErr(err)
	}

	if err := writeReports(outPath, reports); err != nil {
		exitErr(err)
	}
}

func analyze(root string) ([]functionReport, error) {
	serverDir := filepath.Join(root, "server")
	if _, err := os.Stat(serverDir); err != nil {
		return nil, fmt.Errorf("locate server dir: %w", err)
	}
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, serverDir, func(info os.FileInfo) bool {
		name := info.Name()
		if strings.HasSuffix(name, "_test.go") {
			return false
		}
		return strings.HasSuffix(name, ".go")
	}, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse server package: %w", err)
	}

	var infos []*functionInfo
	for _, pkg := range pkgs {
		for filePath, file := range pkg.Files {
			rel, err := filepath.Rel(root, filePath)
			if err != nil {
				rel = filePath
			}
			fileInfos := analyzeFile(fset, rel, file)
			infos = append(infos, fileInfos...)
		}
	}

	sort.Slice(infos, func(i, j int) bool {
		if infos[i].filePath == infos[j].filePath {
			return infos[i].name < infos[j].name
		}
		return infos[i].filePath < infos[j].filePath
	})

	reports := make([]functionReport, 0, len(infos))
	for _, fi := range infos {
		if !isRelevantFunction(fi) {
			continue
		}
		reports = append(reports, fi.toReport())
	}

	return reports, nil
}

func analyzeFile(fset *token.FileSet, relPath string, file *ast.File) []*functionInfo {
	if !isEffectRelatedFile(relPath) {
		return nil
	}
	var infos []*functionInfo
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		info := &functionInfo{
			filePath: relPath,
			name:     fn.Name.Name,

			effectTypes:     make(map[string]struct{}),
			triggerTypes:    make(map[string]struct{}),
			cooldownGuards:  make(map[string]struct{}),
			cooldownSources: make(map[string]struct{}),
			loggingCalls:    make(map[string]struct{}),
			journalCalls:    make(map[string]struct{}),
			mutations:       make(map[string]struct{}),
			categories:      make(map[string]struct{}),
		}
		env := make(map[string]*valueInfo)
		processBlock(fn.Body, env, info)
		infos = append(infos, info)
	}
	return infos
}

func isEffectRelatedFile(path string) bool {
	allowed := map[string]struct{}{
		"server/effects.go":        {},
		"server/conditions.go":     {},
		"server/world_mutators.go": {},
		"server/simulation.go":     {},
	}
	if _, ok := allowed[path]; ok {
		return true
	}
	return false
}

func isRelevantFunction(fi *functionInfo) bool {
	if fi == nil {
		return false
	}
	lower := strings.ToLower(fi.name)
	keywords := []string{
		"effect",
		"trigger",
		"condition",
		"projectile",
		"melee",
		"fireball",
		"environment",
		"burn",
		"blood",
		"aoe",
		"explod",
	}
	matchesKeyword := false
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			matchesKeyword = true
			break
		}
	}
	if !matchesKeyword && len(fi.effectTypes) == 0 && len(fi.triggerTypes) == 0 {
		return false
	}
	if len(fi.categories) == 0 && len(fi.mutations) == 0 && len(fi.journalCalls) == 0 && len(fi.cooldownGuards) == 0 && len(fi.cooldownSources) == 0 && len(fi.loggingCalls) == 0 {
		return false
	}
	return true
}

func processBlock(block *ast.BlockStmt, env map[string]*valueInfo, info *functionInfo) {
	if block == nil {
		return
	}
	for _, stmt := range block.List {
		processStmt(stmt, copyEnv(env), info)
	}
}

func processStmt(stmt ast.Stmt, env map[string]*valueInfo, info *functionInfo) {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		processAssign(s, env, info)
	case *ast.BlockStmt:
		processBlock(s, env, info)
	case *ast.ExprStmt:
		processExpr(s.X, env, info)
	case *ast.IfStmt:
		if s.Init != nil {
			processStmt(s.Init, copyEnv(env), info)
		}
		processExpr(s.Cond, env, info)
		processBlock(s.Body, copyEnv(env), info)
		if s.Else != nil {
			switch els := s.Else.(type) {
			case *ast.BlockStmt:
				processBlock(els, copyEnv(env), info)
			default:
				processStmt(els, copyEnv(env), info)
			}
		}
	case *ast.ForStmt:
		if s.Init != nil {
			processStmt(s.Init, copyEnv(env), info)
		}
		if s.Cond != nil {
			processExpr(s.Cond, env, info)
		}
		if s.Post != nil {
			processStmt(s.Post, copyEnv(env), info)
		}
		processBlock(s.Body, copyEnv(env), info)
	case *ast.RangeStmt:
		if s.Key != nil {
			// clear target from env to avoid leaking outer scope entries
			if ident, ok := s.Key.(*ast.Ident); ok && ident.Name != "_" {
				delete(env, ident.Name)
			}
		}
		if s.Value != nil {
			if ident, ok := s.Value.(*ast.Ident); ok && ident.Name != "_" {
				delete(env, ident.Name)
			}
		}
		processExpr(s.X, env, info)
		processBlock(s.Body, copyEnv(env), info)
	case *ast.ReturnStmt:
		for _, expr := range s.Results {
			processExpr(expr, env, info)
		}
	case *ast.DeferStmt:
		processExpr(s.Call, env, info)
	case *ast.GoStmt:
		processExpr(s.Call, env, info)
	case *ast.IncDecStmt:
		processIncDec(s, info)
	case *ast.SwitchStmt:
		if s.Init != nil {
			processStmt(s.Init, copyEnv(env), info)
		}
		if s.Tag != nil {
			processExpr(s.Tag, env, info)
		}
		for _, stmt := range s.Body.List {
			if cc, ok := stmt.(*ast.CaseClause); ok {
				for _, expr := range cc.List {
					processExpr(expr, env, info)
				}
				processStmtList(cc.Body, copyEnv(env), info)
			}
		}
	case *ast.TypeSwitchStmt:
		if s.Init != nil {
			processStmt(s.Init, copyEnv(env), info)
		}
		if s.Assign != nil {
			processStmt(s.Assign, copyEnv(env), info)
		}
		for _, stmt := range s.Body.List {
			if cc, ok := stmt.(*ast.CaseClause); ok {
				processStmtList(cc.Body, copyEnv(env), info)
			}
		}
	case *ast.SelectStmt:
		for _, comm := range s.Body.List {
			if cc, ok := comm.(*ast.CommClause); ok {
				if cc.Comm != nil {
					processStmt(cc.Comm, copyEnv(env), info)
				}
				processStmtList(cc.Body, copyEnv(env), info)
			}
		}
	}
}

func processStmtList(stmts []ast.Stmt, env map[string]*valueInfo, info *functionInfo) {
	for _, stmt := range stmts {
		processStmt(stmt, copyEnv(env), info)
	}
}

func processAssign(assign *ast.AssignStmt, env map[string]*valueInfo, info *functionInfo) {
	for i, lhs := range assign.Lhs {
		if i >= len(assign.Rhs) {
			break
		}
		if ident, ok := lhs.(*ast.Ident); ok && ident.Name != "_" {
			rhs := assign.Rhs[i]
			vi := extractValueInfo(rhs, env, info)
			if vi != nil {
				env[ident.Name] = vi
			}
		}
	}
	if len(assign.Rhs) == 1 {
		if call, ok := assign.Rhs[0].(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "append" {
				handleAppendCall(call, env, info)
				return
			}
		}
	}
	for _, expr := range assign.Rhs {
		processExpr(expr, env, info)
	}
}

func processIncDec(stmt *ast.IncDecStmt, info *functionInfo) {
	if sel, ok := stmt.X.(*ast.SelectorExpr); ok {
		base := exprToString(sel)
		info.mutations[base+stmt.Tok.String()] = struct{}{}
	}
}

func handleAppendCall(call *ast.CallExpr, env map[string]*valueInfo, info *functionInfo) {
	if len(call.Args) == 0 {
		return
	}
	target := exprToString(call.Args[0])
	if target == "" {
		return
	}
	if strings.HasSuffix(target, ".effects") {
		info.mutations[target+".append"] = struct{}{}
	}
	if strings.HasSuffix(target, ".effectTriggers") {
		info.mutations[target+".append"] = struct{}{}
		info.categories["producer"] = struct{}{}
		info.queueTriggerCall = true
	}
	for _, arg := range call.Args[1:] {
		info.consumeValue(arg, env)
	}
}

func processExpr(expr ast.Expr, env map[string]*valueInfo, info *functionInfo) {
	switch e := expr.(type) {
	case *ast.CallExpr:
		processCallExpr(e, env, info)
	case *ast.CompositeLit:
		vi := extractFromComposite(e, info)
		if vi != nil {
			// anonymous literal in expression
			info.consumeValueFromInfo(vi)
		}
	case *ast.BinaryExpr:
		processExpr(e.X, env, info)
		processExpr(e.Y, env, info)
	case *ast.UnaryExpr:
		processExpr(e.X, env, info)
	case *ast.ParenExpr:
		processExpr(e.X, env, info)
	case *ast.SelectorExpr:
		maybeRecordCooldown(e, info)
	case *ast.IndexExpr:
		processExpr(e.X, env, info)
		processExpr(e.Index, env, info)
	case *ast.SliceExpr:
		processExpr(e.X, env, info)
		if e.Low != nil {
			processExpr(e.Low, env, info)
		}
		if e.High != nil {
			processExpr(e.High, env, info)
		}
		if e.Max != nil {
			processExpr(e.Max, env, info)
		}
	case *ast.TypeAssertExpr:
		processExpr(e.X, env, info)
		if e.Type != nil {
			// nothing additional
		}
	case *ast.Ident:
		maybeRecordCooldownIdent(e, info)
	}
}

func processCallExpr(call *ast.CallExpr, env map[string]*valueInfo, info *functionInfo) {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		if fun.Name == "append" {
			handleAppendCall(call, env, info)
			return
		}
	case *ast.SelectorExpr:
		name := exprToString(fun)
		switch fun.Sel.Name {
		case "QueueEffectTrigger":
			info.queueTriggerCall = true
			info.mutations[name] = struct{}{}
			info.categories["producer"] = struct{}{}
			for _, arg := range call.Args {
				info.consumeValue(arg, env)
			}
		case "SetEffectPosition", "SetEffectParam", "spawnAreaEffectAt", "stopProjectile", "triggerExpiryExplosion", "pruneEffects", "maybeExplodeOnExpiry", "applyEffectHitPlayer", "applyEffectHitNPC", "applyEffectHitActor", "applyCondition", "applyEffectHit", "spawnProjectile", "triggerMeleeAttack", "triggerFireball", "advanceProjectile", "advanceNonProjectiles", "updateFollowEffect", "maybeSpawnBloodSplatter", "applyEnvironmentalConditions", "attachConditionEffect", "applyConditionDamage", "extendAttachedEffect", "expireAttachedEffect":
			info.mutations[name] = struct{}{}
		case "appendPatch", "purgeEntityPatches", "recordJournalEvent":
			info.journalCalls[name] = struct{}{}
		case "cooldownReady":
			info.cooldownGuards[name] = struct{}{}
		}
		if ident, ok := fun.X.(*ast.Ident); ok && strings.HasPrefix(ident.Name, "logging") {
			info.loggingCalls[name] = struct{}{}
		}
	}
	for _, arg := range call.Args {
		processExpr(arg, env, info)
	}
}

func extractValueInfo(expr ast.Expr, env map[string]*valueInfo, info *functionInfo) *valueInfo {
	switch e := expr.(type) {
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			if vi := extractValueInfo(e.X, env, info); vi != nil {
				return vi
			}
		}
	case *ast.CompositeLit:
		return extractFromComposite(e, info)
	case *ast.Ident:
		if vi, ok := env[e.Name]; ok {
			return vi
		}
	}
	processExpr(expr, env, info)
	return nil
}

func (info *functionInfo) consumeValue(expr ast.Expr, env map[string]*valueInfo) {
	if vi := extractValueInfo(expr, env, info); vi != nil {
		info.consumeValueFromInfo(vi)
		return
	}
	processExpr(expr, env, info)
}

func (info *functionInfo) consumeValueFromInfo(vi *valueInfo) {
	for _, effType := range vi.effectTypes {
		info.effectTypes[effType] = struct{}{}
	}
	for _, trigType := range vi.triggerTypes {
		info.triggerTypes[trigType] = struct{}{}
	}
	if len(vi.effectTypes) > 0 {
		info.categories["producer"] = struct{}{}
		info.hasEffectLiteral = true
	}
	if len(vi.triggerTypes) > 0 {
		info.categories["producer"] = struct{}{}
		info.hasTriggerLiteral = true
	}
}

func extractFromComposite(cl *ast.CompositeLit, info *functionInfo) *valueInfo {
	typeName := exprToString(cl.Type)
	vi := &valueInfo{}
	switch {
	case strings.HasSuffix(typeName, "effectState"):
		types := extractEffectTypes(cl)
		if len(types) > 0 {
			vi.effectTypes = types
		}
	case strings.HasSuffix(typeName, "Effect"):
		types := extractEffectLiteralType(cl)
		if len(types) > 0 {
			vi.effectTypes = append(vi.effectTypes, types...)
		}
	case strings.HasSuffix(typeName, "EffectTrigger"):
		types := extractTriggerTypes(cl)
		if len(types) > 0 {
			vi.triggerTypes = types
		}
	}
	if len(vi.effectTypes) == 0 && len(vi.triggerTypes) == 0 {
		return nil
	}
	return vi
}

func extractEffectTypes(cl *ast.CompositeLit) []string {
	var types []string
	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		keyIdent, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch keyIdent.Name {
		case "Effect":
			if nested, ok := kv.Value.(*ast.CompositeLit); ok {
				types = append(types, extractEffectLiteralType(nested)...)
			}
		case "Type":
			if name := exprToTypeName(kv.Value); name != "" {
				types = append(types, name)
			}
		}
	}
	return dedupe(types)
}

func extractEffectLiteralType(cl *ast.CompositeLit) []string {
	for _, elt := range cl.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if ident, ok := kv.Key.(*ast.Ident); ok && ident.Name == "Type" {
				if name := exprToTypeName(kv.Value); name != "" {
					return []string{name}
				}
			}
		}
	}
	return nil
}

func extractTriggerTypes(cl *ast.CompositeLit) []string {
	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if ident, ok := kv.Key.(*ast.Ident); ok && ident.Name == "Type" {
			if name := exprToTypeName(kv.Value); name != "" {
				return []string{name}
			}
		}
	}
	return nil
}

func maybeRecordCooldown(sel *ast.SelectorExpr, info *functionInfo) {
	if strings.Contains(sel.Sel.Name, "Cooldown") {
		info.cooldownSources[exprToString(sel)] = struct{}{}
	}
}

func maybeRecordCooldownIdent(ident *ast.Ident, info *functionInfo) {
	if strings.Contains(ident.Name, "Cooldown") {
		info.cooldownSources[ident.Name] = struct{}{}
	}
}

func exprToTypeName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			return strings.Trim(v.Value, "\"")
		}
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return exprToString(v)
	}
	return ""
}

func exprToString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		left := exprToString(v.X)
		if left == "" {
			return v.Sel.Name
		}
		return left + "." + v.Sel.Name
	case *ast.BasicLit:
		return v.Value
	case *ast.CallExpr:
		return exprToString(v.Fun)
	case *ast.StarExpr:
		return "*" + exprToString(v.X)
	case *ast.IndexExpr:
		return exprToString(v.X)
	case *ast.ParenExpr:
		return exprToString(v.X)
	case *ast.SliceExpr:
		return exprToString(v.X)
	case *ast.UnaryExpr:
		return v.Op.String() + exprToString(v.X)
	}
	return ""
}

func copyEnv(env map[string]*valueInfo) map[string]*valueInfo {
	if len(env) == 0 {
		return make(map[string]*valueInfo)
	}
	clone := make(map[string]*valueInfo, len(env))
	for k, v := range env {
		clone[k] = v
	}
	return clone
}

func dedupe(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		set[v] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func (info *functionInfo) toReport() functionReport {
	report := functionReport{
		File:       info.filePath,
		Function:   info.name,
		Categories: mapKeys(info.categories),
	}
	report.EffectTypes = mapKeys(info.effectTypes)
	report.TriggerTypes = mapKeys(info.triggerTypes)
	report.CooldownGuards = mapKeys(info.cooldownGuards)
	report.CooldownSources = mapKeys(info.cooldownSources)
	report.LoggingCalls = mapKeys(info.loggingCalls)
	report.JournalCalls = mapKeys(info.journalCalls)
	report.Mutations = mapKeys(info.mutations)
	if len(report.Mutations) > 0 && !containsString(report.Categories, "mutation") {
		report.Categories = append(report.Categories, "mutation")
	}
	if len(report.JournalCalls) > 0 && !containsString(report.Categories, "mutation") {
		report.Categories = append(report.Categories, "mutation")
	}
	if len(report.CooldownGuards) > 0 && !containsString(report.Categories, "mutation") {
		report.Categories = append(report.Categories, "mutation")
	}
	sort.Strings(report.Categories)
	report.DeliveryKinds = classifyDeliveryKinds(info)
	return report
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func mapKeys(m map[string]struct{}) []string {
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

func classifyDeliveryKinds(info *functionInfo) []string {
	kinds := make(map[string]struct{})
	lowerName := strings.ToLower(info.name)
	if info.queueTriggerCall || info.hasTriggerLiteral || len(info.triggerTypes) > 0 {
		kinds["trigger"] = struct{}{}
	}
	if strings.Contains(lowerName, "trigger") {
		kinds["trigger"] = struct{}{}
	}
	if strings.Contains(lowerName, "environment") {
		kinds["environment"] = struct{}{}
	}
	if strings.Contains(lowerName, "projectile") || containsType(info.effectTypes, "fireball") {
		kinds["projectile"] = struct{}{}
	}
	if strings.Contains(lowerName, "melee") || containsType(info.effectTypes, "attack") {
		kinds["melee"] = struct{}{}
	}
	if strings.Contains(lowerName, "condition") || containsType(info.effectTypes, "burn") {
		kinds["condition"] = struct{}{}
	}
	if strings.Contains(lowerName, "area") {
		kinds["aoe"] = struct{}{}
	}
	if len(kinds) == 0 {
		if len(info.effectTypes) > 0 {
			kinds["tracked"] = struct{}{}
		} else if info.queueTriggerCall {
			kinds["trigger"] = struct{}{}
		} else if len(info.mutations) > 0 {
			kinds["mutation"] = struct{}{}
		}
	}
	return mapKeys(kinds)
}

func containsType(set map[string]struct{}, needle string) bool {
	if len(set) == 0 || needle == "" {
		return false
	}
	needle = strings.ToLower(needle)
	for value := range set {
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}
	return false
}

func writeReports(path string, reports []functionReport) error {
	data, err := json.MarshalIndent(reports, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure output dir: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
