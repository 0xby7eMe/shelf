package library

import (
	"fmt"
	"strings"
)

type vdfNode map[string]any

func (n vdfNode) str(key string) string {
	s, _ := n[strings.ToLower(key)].(string)
	return s
}

func (n vdfNode) obj(key string) vdfNode {
	o, _ := n[strings.ToLower(key)].(vdfNode)
	return o
}

func (n vdfNode) path(keys ...string) vdfNode {
	cur := n
	for _, k := range keys {
		if cur == nil {
			return nil
		}
		cur = cur.obj(k)
	}
	return cur
}

type tokenKind int

const (
	tokString tokenKind = iota
	tokOpen
	tokClose
	tokEOF
)

type vdfParser struct {
	data []byte
	pos  int
}

func parseVDF(data []byte) (vdfNode, error) {
	p := &vdfParser{data: data}
	return p.parseObject(true)
}

func (p *vdfParser) next() (tokenKind, string, error) {
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			p.pos++
			continue
		}
		if c == '/' && p.pos+1 < len(p.data) && p.data[p.pos+1] == '/' {
			for p.pos < len(p.data) && p.data[p.pos] != '\n' {
				p.pos++
			}
			continue
		}
		break
	}
	if p.pos >= len(p.data) {
		return tokEOF, "", nil
	}

	switch p.data[p.pos] {
	case '{':
		p.pos++
		return tokOpen, "", nil
	case '}':
		p.pos++
		return tokClose, "", nil
	case '"':
		p.pos++
		var sb strings.Builder
		for p.pos < len(p.data) {
			c := p.data[p.pos]
			p.pos++
			switch c {
			case '"':
				return tokString, sb.String(), nil
			case '\\':
				if p.pos < len(p.data) {
					e := p.data[p.pos]
					p.pos++
					switch e {
					case 'n':
						sb.WriteByte('\n')
					case 't':
						sb.WriteByte('\t')
					default:
						sb.WriteByte(e)
					}
				}
			default:
				sb.WriteByte(c)
			}
		}
		return 0, "", fmt.Errorf("unterminated string")
	default:
		start := p.pos
		for p.pos < len(p.data) && !strings.ContainsRune(" \t\r\n{}\"", rune(p.data[p.pos])) {
			p.pos++
		}
		return tokString, string(p.data[start:p.pos]), nil
	}
}

func (p *vdfParser) parseObject(top bool) (vdfNode, error) {
	node := vdfNode{}
	for {
		kind, key, err := p.next()
		if err != nil {
			return nil, err
		}
		switch kind {
		case tokEOF:
			if top {
				return node, nil
			}
			return nil, fmt.Errorf("unexpected end of file")
		case tokClose:
			if top {
				return nil, fmt.Errorf("unexpected '}'")
			}
			return node, nil
		case tokOpen:
			return nil, fmt.Errorf("unexpected '{'")
		}

		kind, val, err := p.next()
		if err != nil {
			return nil, err
		}
		switch kind {
		case tokOpen:
			child, err := p.parseObject(false)
			if err != nil {
				return nil, err
			}
			node[strings.ToLower(key)] = child
		case tokString:
			node[strings.ToLower(key)] = val
		default:
			return nil, fmt.Errorf("missing value for key %q", key)
		}
	}
}