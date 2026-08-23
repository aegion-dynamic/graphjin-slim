package graphql

import (
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"

	"bytes"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/graph"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/util"
)

func (co *Compiler) ParseName(name string) string {
	if co.c.EnableCamelcase {
		return util.ToSnake(name)
	}
	return name
}

func GetQType(t graph.ParserType) qcode.QType {
	switch t {
	case graph.OpQuery:
		return qcode.QTQuery
	case graph.OpSub:
		return qcode.QTSubscription
	case graph.OpMutate:
		return qcode.QTMutation
	default:
		return qcode.QTUnknown
	}
}

func graphNodeToJSON(node *graph.Node, w *bytes.Buffer) {
	switch node.Type {
	case graph.NodeStr:
		w.WriteString(`"` + node.Val + `"`)

	case graph.NodeNum, graph.NodeBool:
		w.WriteString(node.Val)

	case graph.NodeObj:
		w.WriteString(`{`)
		for i, c := range node.Children {
			if i == 0 {
				w.WriteString(`"` + c.Name + `": `)
			} else {
				w.WriteString(`,"` + c.Name + `": `)
			}
			graphNodeToJSON(c, w)
		}
		w.WriteString(`}`)

	case graph.NodeList:
		w.WriteString(`[`)
		for i, c := range node.Children {
			if i != 0 {
				w.WriteString(`,`)
			}
			graphNodeToJSON(c, w)
		}
		w.WriteString(`]`)
	}
}
