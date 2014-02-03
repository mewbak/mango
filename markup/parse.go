package markup

import (
	"errors"
)

const (
	NODE_GROUP = iota
	NODE_BLOCK
	NODE_SECTION
	NODE_TEXT
	NODE_TEXTBOLD
	NODE_TEXTUNDERLINE
	NODE_LIST
	NODE_LISTITEM
	NODE_SPACE
	NODE_BREAK
)

var (
	ErrEOF       = errors.New("eof")
	ErrIndent    = errors.New("indentation mismatch")
	ErrRange     = errors.New("out of range")
	ErrMalformed = errors.New("malformed syntax")
)

type Node struct {
	Kind   int
	Text   string
	Parent *Node
	Childs []*Node
}

func NewNode(kind int) *Node {
	return NewNodeWithText(kind, "")
}

func NewNodeWithText(kind int, text string) *Node {
	return &Node{kind, text, nil, make([]*Node, 0)}
}

func (n *Node) String() string {
	switch n.Kind {
	case NODE_GROUP:
		return "Group"
	case NODE_BLOCK:
		return "Block"
	case NODE_SECTION:
		return "Section"
	case NODE_TEXT:
		return "Text"
	case NODE_TEXTBOLD:
		return "TextBold"
	case NODE_TEXTUNDERLINE:
		return "TextUnderline"
	case NODE_LIST:
		return "List"
	case NODE_LISTITEM:
		return "ListItem"
	}
	return "Unkown"
}

func (n *Node) AddChild(c *Node) {
	c.Parent = n
	n.Childs = append(n.Childs, c)
}

func (n *Node) IsTextNode() bool {
	switch n.Kind {
	case NODE_TEXT, NODE_TEXTBOLD, NODE_TEXTUNDERLINE:
		return true
	default:
		return false
	}
}

type Parser struct {
	root  *Node
	curr  *Node
	onAdd func(*Parser, *Node) // Function get's called before the next node
	// get's added to the current node.
}

type TokenGroup struct {
	tokens []*Token
	level  int
	pos    int
}

func NewTokenGroup(inpTokens []*Token) (*TokenGroup, int) {
	// Detect indent level, this means: count and skip all leading indents.
	level := 0
	for level < len(inpTokens) {
		if !inpTokens[level].Is(TOKEN_INDENT) {
			break
		}
		level++
	}
	// Consume all tokens until we reach a TOKEN_EOL
	tokens := make([]*Token, 0)
	for _, token := range inpTokens[level:] {
		if token.Is(TOKEN_EOL) {
			break
		}
		tokens = append(tokens, token)
	}
	// Return a new group and the number of tokens consumed (including indents)
	return &TokenGroup{tokens, level, 0}, level + len(tokens)
}

// Return a slice of tokens starting at the current position.
func (g *TokenGroup) Tokens() Tokens {
	i := g.pos
	if i >= len(g.tokens) {
		i = len(g.tokens)
	}
	return Tokens(g.tokens[i:])
}

// Return the current token and advance to the next. Returns nil if there are
// no tokens left.
func (g *TokenGroup) Next() *Token {
	if g.pos >= len(g.tokens) {
		return nil
	}
	result := g.tokens[g.pos]
	g.pos++
	return result
}

func NewParser() *Parser {
	return new(Parser)
}

func (p *Parser) addNode(kind int, text ...string) *Node {
	var node *Node = nil
	if len(text) == 0 {
		node = NewNode(kind)
	} else {
		node = NewNodeWithText(kind, text[0])
	}

	if callback := p.onAdd; callback != nil {
		p.onAdd = nil
		callback(p, node)
	}
	p.curr.AddChild(node)
	return node
}

func (p *Parser) lastNode() *Node {
	if len(p.curr.Childs) > 0 {
		return p.curr.Childs[len(p.curr.Childs)-1]
	}
	return p.curr
}

// Open a new group. All nodes will be added to the newly created group.
func (p *Parser) openGroup() {
	p.curr = p.addNode(NODE_GROUP)
}

// Close the parent group (including opened list items + lists).
func (p *Parser) closeGroup() {
	// Close all the other stuff until we reach the first group node.
	for ; p.curr.Parent != nil; p.curr = p.curr.Parent {
		if p.curr.Kind == NODE_GROUP {
			break
		}
	}
	if p.curr.Parent != nil {
		p.curr = p.curr.Parent
	}
}

// Close all groups up to root node.
func (p *Parser) closeAllGroups() {
	p.curr = p.root
}

func (p *Parser) GetRoot() *Node {
	return p.root
}

func (p *Parser) Parse(tokens []*Token) *Node {
	p.root = NewNode(NODE_GROUP)
	p.curr = p.root

	if len(tokens) == 0 {
		return p.root
	}

	// Split tokens into groups.
	groups := make([]*TokenGroup, 0)
	for len(tokens) > 0 {
		group, consumed := NewTokenGroup(tokens)

		tokens = tokens[consumed+1:]
		groups = append(groups, group)
	}

	lastLevel := 0
	for i, group := range groups {
		// An empty line closes all opened levels.
		if len(group.tokens) == 0 && i > 0 {
			p.closeAllGroups()
			p.addNode(NODE_BREAK)
			continue
		}

		// Close / open groups if the line level changed.
		switch levelDiff := group.level - lastLevel; {
		case levelDiff > 0:
			for ; levelDiff > 0; levelDiff-- {
				p.openGroup()
			}
		case levelDiff < 0:
			for ; levelDiff < 0; levelDiff++ {
				p.closeGroup()
			}
		}

		if group.Tokens().Are(TOKEN_BLOCKITEM) {
			item := group.Next()
			if p.curr.Kind != NODE_BLOCK {
				p.curr = p.addNode(NODE_BLOCK)
			}
			p.addNode(NODE_TEXT, item.Text)
		}

		// Check if the current line is a list item.
		if group.Tokens().Are(TOKEN_LISTITEM) {
			item := group.Next()
			// We want to add the listitem node to a list node.
			// There a three possibilities:
			switch p.curr.Kind {
			case NODE_LISTITEM:
				// The parent node is a list item. Close and replace it with
				// it's parent, the list it belongs to.
				p.curr = p.curr.Parent
			case NODE_LIST:
				// The parent node is a list. There's nothing we need to do.
				break
			default:
				// The parent node is neither a list nor a listitem.
				// We have to create a new list node.
				p.curr = p.addNode(NODE_LIST)
			}
			p.curr = p.addNode(NODE_LISTITEM, item.Text)
		}

		if p.lastNode().IsTextNode() {
			p.onAdd = func(p *Parser, n *Node) {
				if n.IsTextNode() {
					p.addNode(NODE_SPACE)
				}
			}
		}

		for {
			token := group.Next()
			if token == nil {
				break
			}
			switch token.Kind {
			case TOKEN_SECTION:
				p.closeAllGroups()
				p.addNode(NODE_SECTION, token.Text)
			case TOKEN_TEXT:
				p.addNode(NODE_TEXT, token.Text)
			case TOKEN_STAR:
				if group.Tokens().Are(TOKEN_TEXT, TOKEN_STAR) {
					text, _ := group.Next(), group.Next()
					p.addNode(NODE_TEXTBOLD, text.Text)
				}
			case TOKEN_UNDERLINE:
				if group.Tokens().Are(TOKEN_TEXT, TOKEN_UNDERLINE) {
					text, _ := group.Next(), group.Next()
					p.addNode(NODE_TEXTUNDERLINE, text.Text)
				}
			}
		}

		if p.onAdd != nil {
			p.onAdd = nil
		}

		lastLevel = group.level
	}
	return p.root
}