package element

import (
	"sort"

	"webbuilder/internal/domain/entity"
)


func ParentsEqual(p1, p2 *string) bool {
	if p1 == nil && p2 == nil {
		return true
	}
	if p1 == nil || p2 == nil {
		return false
	}
	return *p1 == *p2
}

func CompareParent(p *string, id string) bool {
	if p == nil {
		return id == ""
	}
	return *p == id
}

// FlattenElements flattens nested Elements for database operations.
func FlattenElements(elements []entity.Element) []entity.Element {
	result := make([]entity.Element, 0, len(elements))
	flatten(elements, &result)
	return result
}

func flatten(elements []entity.Element, result *[]entity.Element) {
	if len(elements) == 0 {
		return
	}
	for _, el := range elements {
		*result = append(*result, el)
		if len(el.Elements) > 0 {
			flatten(el.Elements, result)
		}
	}
}

// BuildElementTree converts a flat list of elements into a hierarchical tree based on ParentID.
func BuildElementTree(elements []entity.Element) []entity.Element {
	if len(elements) == 0 {
		return nil
	}

	elementMap := make(map[string]entity.Element, len(elements))
	for _, el := range elements {
		el.Elements = nil
		elementMap[el.ID] = el
	}

	childrenMap := make(map[string][]string)
	roots := make([]string, 0)

	for _, el := range elements {
		if el.ParentID == nil || *el.ParentID == "" {
			roots = append(roots, el.ID)
			continue
		}

		pID := *el.ParentID
		if _, ok := elementMap[pID]; ok {
			childrenMap[pID] = append(childrenMap[pID], el.ID)
		} else {
			roots = append(roots, el.ID)
		}
	}

	for pid, cids := range childrenMap {
		sort.SliceStable(cids, func(i, j int) bool {
			a := elementMap[cids[i]]
			b := elementMap[cids[j]]
			if a.Order != b.Order {
				return a.Order < b.Order
			}
			return a.ID < b.ID
		})
		childrenMap[pid] = cids
	}

	sort.SliceStable(roots, func(i, j int) bool {
		a := elementMap[roots[i]]
		b := elementMap[roots[j]]
		if a.Order != b.Order {
			return a.Order < b.Order
		}
		return a.ID < b.ID
	})

	var build func(id string) entity.Element
	build = func(id string) entity.Element {
		node := elementMap[id]
		for _, cid := range childrenMap[id] {
			node.Elements = append(node.Elements, build(cid))
		}
		return node
	}

	result := make([]entity.Element, 0, len(roots))
	for _, rid := range roots {
		result = append(result, build(rid))
	}
	return result
}
