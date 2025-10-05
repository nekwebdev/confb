package blend

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/nekwebdev/confb/internal/config"
)

// BlendKDL merges KDL fragments according to rules (keys + optional section_keys)
// Blocks may have identifier arguments (the "head"), e.g. `output "DP-2" { ... }`.
// Merge occurs only between blocks with the SAME name and SAME head.
func BlendKDL(rules *config.MergeRules, files []string) (string, error) {
	if rules == nil {
		return "", fmt.Errorf("merge rules required")
	}

	// prepare merge-eligible section name set
	mergeAll := true
	eligible := map[string]struct{}{}
	if len(rules.KDLSectionKeys) > 0 {
		mergeAll = false
		for _, k := range rules.KDLSectionKeys {
			eligible[k] = struct{}{}
		}
	}

	// root aggregator
	root := newNode("__root__", "")

	// parse + merge each file in order
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %q: %w", path, err)
		}
		top, err := parseKDL(string(b))
		if err != nil {
			return "", fmt.Errorf("%s: %w", path, err)
		}

		// for each top-level section: merge or append
		for _, childName := range top.ChildrenOrder {
			list := top.Children[childName]
			for _, inst := range list {
				if mergeAll || isEligible(childName, eligible) {
					// merge into first existing instance with same (name, head), or create one
					dst := root.ensureSingle(childName, inst.Head)
					dst.mergeFrom(inst, rules)
				} else {
					// keep separate instance
					root.appendChild(childName, inst.clone())
				}
			}
		}
	}

	// render deterministically
	return root.renderKDL(0), nil
}

func isEligible(name string, set map[string]struct{}) bool {
	_, ok := set[name]
	return ok
}

// --- KDL node model & parser ---

type node struct {
	Name          string
	Head          string                      // raw arguments after identifier, before '{' (e.g., `"DP-2"`)
	Props         map[string][]string         // key -> list of values (to support append)
	PropsOrder    []string                    // capture seen keys; rendered sorted for determinism
	Children      map[string][]*node          // section name -> instances (each has its own Head)
	ChildrenOrder []string                    // stable order of child names; rendered sorted
}

func newNode(name, head string) *node {
	return &node{
		Name:          name,
		Head:          head,
		Props:         map[string][]string{},
		PropsOrder:    []string{},
		Children:      map[string][]*node{},
		ChildrenOrder: []string{},
	}
}

func (n *node) clone() *node {
	cp := newNode(n.Name, n.Head)
	for k, vs := range n.Props {
		cp.Props[k] = append([]string(nil), vs...)
	}
	cp.PropsOrder = append([]string(nil), n.PropsOrder...)
	for k, list := range n.Children {
		for _, c := range list {
			cp.appendChild(k, c.clone())
		}
	}
	return cp
}

// ensureSingle: find first child with same (name, head), else create.
func (n *node) ensureSingle(name, head string) *node {
	if lst, ok := n.Children[name]; ok && len(lst) > 0 {
		for _, cand := range lst {
			if cand.Head == head {
				return cand
			}
		}
	}
	child := newNode(name, head)
	n.appendChild(name, child)
	return child
}

func (n *node) appendChild(name string, c *node) {
	if _, ok := n.Children[name]; !ok {
		n.Children[name] = []*node{}
		n.ChildrenOrder = append(n.ChildrenOrder, name)
	}
	n.Children[name] = append(n.Children[name], c)
}

func (n *node) setProp(key, val string, mode string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	switch mode {
	case "first_wins":
		if _, exists := n.Props[key]; !exists {
			n.Props[key] = []string{val}
			n.PropsOrder = append(n.PropsOrder, key)
		}
	case "append":
		if _, exists := n.Props[key]; !exists {
			n.PropsOrder = append(n.PropsOrder, key)
		}
		n.Props[key] = append(n.Props[key], val)
	default: // last_wins
		if _, exists := n.Props[key]; !exists {
			n.PropsOrder = append(n.PropsOrder, key)
		}
		n.Props[key] = []string{val}
	}
}

func (dst *node) mergeFrom(src *node, rules *config.MergeRules) {
	// merge props
	mode := strings.ToLower(rules.KDLKeys)
	for k, vs := range src.Props {
		for _, v := range vs {
			dst.setProp(k, v, mode)
		}
	}
	// merge children: always coalesce by (name, head) inside a merged section
	for name, instances := range src.Children {
		for _, inst := range instances {
			child := dst.ensureSingle(name, inst.Head)
			child.mergeFrom(inst, rules)
		}
	}
}

// renderKDL prints children in lexicographic name order; props keys sorted lex.
// Two-space indentation.
func (n *node) renderKDL(depth int) string {
	if n.Name == "__root__" {
		var sections []string
		names := append([]string(nil), n.ChildrenOrder...)
		sort.Strings(names)
		for _, name := range names {
			for _, c := range n.Children[name] {
				sections = append(sections, c.renderKDL(depth))
			}
		}
		out := strings.Join(sections, "")
		if !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		return out
	}

	indent := strings.Repeat("  ", depth)
	var b strings.Builder
	b.WriteString(indent)
	b.WriteString(n.Name)
	if strings.TrimSpace(n.Head) != "" {
		b.WriteString(" ")
		b.WriteString(n.Head)
	}
	b.WriteString(" {\n")

	// props sorted by key for determinism
	keys := append([]string(nil), n.PropsOrder...)
	sort.Strings(keys)
	for _, k := range keys {
		vs := n.Props[k]
		for _, v := range vs {
			b.WriteString(indent)
			b.WriteString("  ")
			if v == "" {
				b.WriteString(k)
				b.WriteString("\n")
			} else {
				b.WriteString(k)
				b.WriteString(" ")
				b.WriteString(v)
				b.WriteString("\n")
			}
		}
	}

	// children sorted by name
	chNames := append([]string(nil), n.ChildrenOrder...)
	sort.Strings(chNames)
	for _, name := range chNames {
		for _, c := range n.Children[name] {
			b.WriteString(c.renderKDL(depth + 1))
		}
	}

	b.WriteString(indent)
	b.WriteString("}\n")
	return b.String()
}

// --- parser ---

// Very small parser: recognizes blocks "ident [args...] {" and nested scopes.
// Inside a block, any non-`}` / non-block-start line is a property "key value..." (raw).
// Comments starting with '//' are stripped. Strings/escaping are not fully parsed; args and values are kept raw.
func parseKDL(s string) (*node, error) {
	s = stripLineComments(s)
	r := bufio.NewReader(strings.NewReader(s))
	root := newNode("__root__", "")
	var stack []*node
	cur := root

	for {
		line, err := readLogicalLine(r)
		if line == "" && err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			if err != nil {
				break
			}
			continue
		}

		// Closing brace?
		if line == "}" {
			if len(stack) == 0 {
				return nil, fmt.Errorf("unmatched closing brace")
			}
			cur = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if err != nil {
				break
			}
			continue
		}

		// Block start?  <ident> [args...] {
		if name, head, ok := isBlockStart(line); ok {
			child := newNode(name, head)
			cur.appendChild(name, child)
			stack = append(stack, cur)
			cur = child
			if err != nil {
				break
			}
			continue
		}

		// Otherwise it's a prop: split first token as key, rest as value (kept raw)
		key, val := splitFirstToken(line)
		cur.setProp(key, val, "append") // merge policy applied later
		if err != nil {
			break
		}
	}

	if len(stack) != 0 {
		return nil, fmt.Errorf("unclosed block(s)")
	}
	return root, nil
}

func stripLineComments(s string) string {
	var out []string
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := sc.Text()
		// drop everything after '//' (naive; good enough for MVP)
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func readLogicalLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	return line, err
}

// isBlockStart accepts lines like:
//   output {          -> name="output", head=""
//   output "DP-2" {   -> name="output", head="\"DP-2\""
// We keep head as raw text (no parsing of strings/escapes).
func isBlockStart(line string) (name, head string, ok bool) {
	line = strings.TrimSpace(line)
	if !strings.HasSuffix(line, "{") {
		return "", "", false
	}
	left := strings.TrimSpace(strings.TrimSuffix(line, "{"))
	if left == "" {
		return "", "", false
	}
	// name is first token; head is remainder (may be empty)
	i := strings.IndexAny(left, " \t")
	if i < 0 {
		return left, "", true
	}
	name = strings.TrimSpace(left[:i])
	head = strings.TrimSpace(left[i+1:])
	if name == "" {
		return "", "", false
	}
	// allow any head; we just keep it raw
	return name, head, true
}

func splitFirstToken(line string) (string, string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", ""
	}
	// key is up to first space; rest (if any) is value (kept raw)
	i := strings.IndexAny(line, " \t")
	if i < 0 {
		return line, ""
	}
	key := strings.TrimSpace(line[:i])
	val := strings.TrimSpace(line[i+1:])
	return key, val
}
