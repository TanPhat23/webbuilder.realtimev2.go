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

func UpdateElement(root *entity.Element, updated entity.Element) bool {
	if root == nil {
		return false
	}
	if root.ID == updated.ID {
		*root = updated
		return true
	}
	for i := range root.Elements {
		if UpdateElement(&root.Elements[i], updated) {
			return true
		}
	}
	return false
}

func DeleteElement(root *entity.Element, id string) bool {
	if root == nil {
		return false
	}
	for i := 0; i < len(root.Elements); i++ {
		if root.Elements[i].ID == id {
			root.Elements = append(root.Elements[:i], root.Elements[i+1:]...)
			return true
		}
	}
	for i := range root.Elements {
		if DeleteElement(&root.Elements[i], id) {
			return true
		}
	}
	return false
}

func SwapElements(root *entity.Element, id1, id2 string) bool {
	if root == nil {
		return false
	}
	var idx1, idx2 = -1, -1
	for i := range root.Elements {
		if root.Elements[i].ID == id1 {
			idx1 = i
		} else if root.Elements[i].ID == id2 {
			idx2 = i
		}
	}
	if idx1 != -1 && idx2 != -1 {
		root.Elements[idx1], root.Elements[idx2] = root.Elements[idx2], root.Elements[idx1]
		return true
	}

	for i := range root.Elements {
		if SwapElements(&root.Elements[i], id1, id2) {
			return true
		}
	}
	return false
}

func CreateElement(root *entity.Element, newElement entity.Element) bool {
	if root == nil {
		return false
	}
	if (newElement.ParentID == nil && root.ID == "") || (newElement.ParentID != nil && *newElement.ParentID == root.ID) {
		root.Elements = append(root.Elements, newElement)
		return true
	}
	for i := range root.Elements {
		if CreateElement(&root.Elements[i], newElement) {
			return true
		}
	}
	return false
}
